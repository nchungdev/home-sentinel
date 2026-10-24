import json
import os
import re
import urllib.parse
import urllib.request

PLEX_PREF = (
    "/appdata/plex/database/Library/Application Support/"
    "Plex Media Server/Preferences.xml"
)
RADARR_CONF = "/appdata/radarr/config.xml"
SONARR_CONF = "/appdata/sonarr/config.xml"
NAS_HOST = os.environ.get("AGENT_NAS_IP", "127.0.0.1")
PLEX_MACHINE_ID = os.environ.get("PLEX_MACHINE_ID", "")


def read_plex_machine_id():
    if PLEX_MACHINE_ID:
        return PLEX_MACHINE_ID
    try:
        with open(PLEX_PREF, "r", encoding="utf-8") as file:
            match = re.search(r'ProcessedMachineIdentifier="([^"]+)"', file.read())
            if match:
                return match.group(1)
    except Exception:
        pass
    return ""


def read_plex_token():
    try:
        with open(PLEX_PREF, "r", encoding="utf-8") as file:
            match = re.search(r'PlexOnlineToken="([^"]+)"', file.read())
            return match.group(1) if match else None
    except OSError:
        return None


def read_api_key(config_path):
    try:
        with open(config_path, "r", encoding="utf-8") as file:
            match = re.search(r"<ApiKey>([^<]+)</ApiKey>", file.read())
            return match.group(1) if match else None
    except OSError:
        return None


def clean_query(query):
    cleaned = re.sub(
        r"(?i)\b(phim|bộ|lẻ|hoạt\s*hình|anime|search|tìm|xem|kiếm)\b", "", query
    )
    cleaned = re.sub(
        r"(?i)\s+(pirood|period|pro|full|vietsub|thuyết\s*minh|tập|season|phần).*$",
        "",
        cleaned,
    )
    return cleaned.strip()


def resolve_plex_thumb(thumb_path, token):
    if not thumb_path:
        return ""
    url = f"http://127.0.0.1:32400{thumb_path}?X-Plex-Token={token}"
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, req, fp, code, msg, headers, newurl):
            return None
    opener = urllib.request.build_opener(NoRedirect)
    try:
        opener.open(url, timeout=1.5)
    except urllib.error.HTTPError as err:
        if err.code in (301, 302):
            loc = err.headers.get("Location")
            if loc:
                return loc
    except Exception:
        pass
    return f"http://{NAS_HOST}:32400{thumb_path}?X-Plex-Token={token}"


def plex_search(query):
    token = read_plex_token()
    if not token:
        return []
    cleaned_query = clean_query(query) or query
    url = (
        f"http://127.0.0.1:32400/hubs/search?"
        f"query={urllib.parse.quote(cleaned_query)}&X-Plex-Token={token}"
    )
    request = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=3) as response:
            data = json.loads(response.read().decode("utf-8"))
            results = []
            for hub in data.get("MediaContainer", {}).get("Hub", []):
                for item in hub.get("Metadata", []):
                    item_type = item.get("type", "movie")
                    if item_type not in ("movie", "show"):
                        continue
                    title = item.get("title", "")
                    rating_key = item.get("ratingKey", "")
                    thumb = item.get("thumb")
                    thumb_url = resolve_plex_thumb(thumb, token)
                    results.append({
                        "title": title,
                        "ratingKey": rating_key,
                        "year": item.get("year", ""),
                        "type": "Movie" if item_type == "movie" else "Series",
                        "summary": item.get("summary", ""),
                        "seasons": item.get("childCount"),
                        "episodes": item.get("leafCount"),
                        "poster": thumb_url,
                        "source": "plex",
                    })
            return results
    except Exception:
        return []


def radarr_lookup(query):
    api_key = read_api_key(RADARR_CONF)
    if not api_key:
        return []
    cleaned_query = clean_query(query) or query
    url = (
        f"http://127.0.0.1:7878/api/v3/movie/lookup?"
        f"term={urllib.parse.quote(cleaned_query)}&apiKey={api_key}"
    )
    request = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=3) as response:
            data = json.loads(response.read().decode("utf-8"))
            results = []
            for item in data[:4]:
                poster = ""
                for img in item.get("images", []):
                    if img.get("coverType") == "poster":
                        poster = img.get("remoteUrl", "")
                        break
                results.append({
                    "title": item.get("title", ""),
                    "year": item.get("year", ""),
                    "type": "Movie",
                    "inLibrary": item.get("inLibrary", False),
                    "summary": item.get("overview", ""),
                    "poster": poster,
                    "source": "radarr",
                    "raw": item,
                })
            return results
    except Exception:
        return []


def sonarr_lookup(query):
    api_key = read_api_key(SONARR_CONF)
    if not api_key:
        return []
    cleaned_query = clean_query(query) or query
    url = (
        f"http://127.0.0.1:8989/api/v3/series/lookup?"
        f"term={urllib.parse.quote(cleaned_query)}&apiKey={api_key}"
    )
    request = urllib.request.Request(url, headers={"Accept": "application/json"})
    try:
        with urllib.request.urlopen(request, timeout=3) as response:
            data = json.loads(response.read().decode("utf-8"))
            results = []
            for item in data[:4]:
                poster = ""
                for img in item.get("images", []):
                    if img.get("coverType") == "poster":
                        poster = img.get("remoteUrl", "")
                        break
                results.append({
                    "title": item.get("title", ""),
                    "year": item.get("year", ""),
                    "type": "Series",
                    "inLibrary": item.get("inLibrary", False),
                    "summary": item.get("overview", ""),
                    "seasons": item.get("seasonCount"),
                    "poster": poster,
                    "source": "sonarr",
                    "raw": item,
                })
            return results
    except Exception:
        return []


def render_media_results(query):
    plex_items = plex_search(query)
    radarr_items = radarr_lookup(query)
    sonarr_items = sonarr_lookup(query)

    items = []
    seen = set()

    for item in plex_items:
        key = (item["title"].lower(), str(item.get("year", "")))
        if key not in seen:
            seen.add(key)
            rating_key = item.get("ratingKey", "")
            machine_id = read_plex_machine_id()
            if machine_id:
                deep_link = (
                    f"plex://server/{machine_id}/com.plexapp.plugins.library?"
                    f"key=%2Flibrary%2Fmetadata%2F{rating_key}"
                )
            else:
                deep_link = f"plex://play?key=%2Flibrary%2Fmetadata%2F{rating_key}"
            items.append({
                "title": item["title"],
                "year": item.get("year", ""),
                "type": item["type"],
                "status": "plex",
                "seasons": item.get("seasons"),
                "episodes": item.get("episodes"),
                "poster": item.get("poster", ""),
                "link": deep_link,
            })

    for item in sonarr_items + radarr_items:
        key = (item["title"].lower(), str(item.get("year", "")))
        if key not in seen:
            seen.add(key)
            status = "downloading" if item.get("inLibrary") else "missing"
            items.append({
                "title": item["title"],
                "year": item.get("year", ""),
                "type": item["type"],
                "status": status,
                "seasons": item.get("seasons"),
                "poster": item.get("poster", ""),
                "link": "",
            })

    if not items:
        return None

    cards_html = []
    for item in items[:6]:
        title = item["title"]
        year_val = item.get("year")
        item_type = item["type"]
        icon = "🎬" if item_type == "Movie" else "📺"

        sub_parts = [icon]
        if year_val:
            sub_parts.append(str(year_val))
        if item_type == "Series" and item.get("seasons"):
            sub_parts.append(f"{item['seasons']} Season")
        sub_text = " • ".join(sub_parts)

        poster_html = (
            f'<img class="poster" src="{item["poster"]}" alt="{title}">'
            if item.get("poster") else
            '<div class="poster"></div>'
        )

        if item["status"] == "plex":
            source_tag = '<span class="source-tag source-plex">PLEX</span>'
            server_label = '<span class="source-server">duinch</span>'
        elif item["status"] == "downloading":
            source_tag = '<span class="source-tag source-tmdb">TMDB</span>'
            server_label = '<span class="source-server">Đang tải</span>'
        else:
            source_tag = '<span class="source-tag source-tmdb">TMDB</span>'
            server_label = '<span class="source-server">Chưa có trên NAS</span>'

        clean_title = title.replace('"', '&quot;')
        card = (
            f'<div class="card" '
            f'data-status="{item["status"]}" '
            f'data-title="{clean_title}" '
            f'data-type="{item_type}" '
            f'data-year="{year_val}" '
            f'data-link="{item["link"]}">'
            f'{poster_html}'
            f'<div class="meta">'
            f'<h3 class="meta-title">{title}</h3>'
            f'<div class="meta-sub">{sub_text}</div>'
            f'<div class="meta-source">{source_tag} {server_label}</div>'
            f'</div>'
            f'</div>'
        )
        cards_html.append(card)

    return f'<div class="grid">{"".join(cards_html)}</div>'


def add_to_radarr(query):
    api_key = read_api_key(RADARR_CONF)
    if not api_key:
        return "Không tìm thấy cấu hình Radarr trên máy chủ."
    movies = radarr_lookup(query)
    if not movies:
        return f"Không tìm thấy phim '{query}' trên cơ sở dữ liệu."
    chosen = movies[0]
    if chosen.get("inLibrary"):
        return f"Phim '{chosen['title']}' đã có sẵn trong danh sách tải của Radarr."
    payload = {
        **chosen["raw"],
        "qualityProfileId": 1,
        "rootFolderPath": "/movies",
        "monitored": True,
        "addOptions": {"searchForMovie": True},
    }
    url = f"http://127.0.0.1:7878/api/v3/movie?apiKey={api_key}"
    request = urllib.request.Request(
        url,
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=5) as response:
            if response.status in (200, 201):
                return (
                    f"Đã nạp '{chosen['title']} ({chosen['year']})' vào Radarr. "
                    "Hệ thống đang tự động tìm nguồn và tải về NAS."
                )
    except Exception as error:
        return f"Lỗi khi nạp vào Radarr: {error}"
    return "Không thể nạp phim vào lúc này."


def add_to_sonarr(query):
    api_key = read_api_key(SONARR_CONF)
    if not api_key:
        return "Không tìm thấy cấu hình Sonarr trên máy chủ."
    series = sonarr_lookup(query)
    if not series:
        return f"Không tìm thấy series '{query}' trên cơ sở dữ liệu."
    chosen = series[0]
    if chosen.get("inLibrary"):
        return f"Series '{chosen['title']}' đã có sẵn trong Sonarr."
    payload = {
        **chosen["raw"],
        "qualityProfileId": 1,
        "rootFolderPath": "/tv",
        "monitored": True,
        "addOptions": {
            "searchForMissingEpisodes": True,
        },
    }
    url = f"http://127.0.0.1:8989/api/v3/series?apiKey={api_key}"
    request = urllib.request.Request(
        url,
        data=json.dumps(payload).encode("utf-8"),
        headers={"Content-Type": "application/json"},
    )
    try:
        with urllib.request.urlopen(request, timeout=5) as response:
            if response.status in (200, 201):
                return (
                    f"Đã nạp '{chosen['title']} ({chosen['year']})' vào Sonarr. "
                    "Hệ thống đang tự động tìm kiếm các tập phim."
                )
    except Exception as error:
        return f"Lỗi khi nạp vào Sonarr: {error}"
    return "Không thể nạp series vào lúc này."
