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
            status = "downloading" if item.get("inLibrary") or item.get("raw", {}).get("id", 0) > 0 else "missing"
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


def arr_request(service, path, body=None):
    config = RADARR_CONF if service == 'RADARR' else SONARR_CONF
    key = os.environ.get(service + '_API_KEY') or read_api_key(config)
    if not key:
        raise ValueError('Chưa có API key ' + service)
    default = 'http://127.0.0.1:7878' if service == 'RADARR' else 'http://127.0.0.1:8989'
    base = os.environ.get(service + '_URL', default)
    request = urllib.request.Request(
        base.rstrip('/') + '/api/v3/' + path,
        data=None if body is None else json.dumps(body).encode(),
        headers={'X-Api-Key': key, 'Content-Type': 'application/json'})
    with urllib.request.urlopen(request, timeout=10) as response:
        return json.load(response)


def select_arr_item(items, query, identity):
    explicit = re.fullmatch(r'(?:tmdb|tvdb):(\d+)', query.strip(), re.I)
    if explicit:
        return next((item for item in items if item.get(identity) == int(explicit.group(1))), None)
    match = re.fullmatch(r'(.*?)\s*\(?((?:19|20)\d{2})\)?', query.strip())
    name, year = (match.group(1).strip(), int(match.group(2))) if match else (query.strip(), None)
    exact = [item for item in items if
             name.casefold() in (item.get('title', '').casefold(),
                                 item.get('originalTitle', '').casefold())
             and (year is None or item.get('year') == year)]
    return exact[0] if len(exact) == 1 else None


def add_arr(query, service):
    kind, identity = ('movie', 'tmdbId') if service == 'RADARR' else ('series', 'tvdbId')
    try:
        query = re.sub(r'\s+(?:vào|vao|to)\s+(?:radarr|sonarr)\s*$', '', query, flags=re.I)
        items = arr_request(service, kind + '/lookup?term=' + urllib.parse.quote(query))
        chosen = select_arr_item(items, query, identity)
        if chosen is None:
            options = '; '.join(f'{i.get("title")} ({i.get("year")}), '
                                f'{"tmdb" if kind == "movie" else "tvdb"}:{i.get(identity)}'
                                for i in items[:5])
            return 'Chưa xác định duy nhất phim. Hãy nêu tên và năm hoặc mã ID: ' + options
        existing = arr_request(service, kind)
        if any(item.get(identity) == chosen[identity] for item in existing):
            return f'{chosen["title"]} đã có trong {service}; không thêm trùng.'
        roots = arr_request(service, 'rootfolder')
        profiles = arr_request(service, 'qualityprofile')
        root = os.environ.get(service + '_ROOT_FOLDER')
        profile = os.environ.get(service + '_QUALITY_PROFILE_ID')
        if not root and len(roots) == 1:
            root = roots[0]['path']
        if not profile and len(profiles) == 1:
            profile = profiles[0]['id']
        if root not in [item['path'] for item in roots] or not profile or \
                int(profile) not in [item['id'] for item in profiles]:
            return 'Cần cấu hình root folder và quality profile ' + service + ' trước khi thêm.'
        payload = dict(chosen, rootFolderPath=root, qualityProfileId=int(profile), monitored=True)
        if kind == 'series':
            payload['seasonFolder'] = True
        payload['addOptions'] = ({'searchForMovie': True} if kind == 'movie' else
                                 {'searchForMissingEpisodes': True})
        added = arr_request(service, kind, payload)
        verified = arr_request(service, kind + '/' + str(added['id']))
        if verified.get(identity) != chosen[identity]:
            return 'API nhận lệnh nhưng chưa xác minh được bản ghi vừa thêm.'
        return (f'Đã thêm {verified["title"]} ({verified.get("year")}) vào {service} '
                'và yêu cầu tìm nguồn. Chưa xác nhận đã tải được phim.')
    except (OSError, ValueError, KeyError, TypeError):
        return 'Không thêm được phim. Kiểm tra kết nối và cấu hình ' + service + '.'


def add_to_radarr(query):
    return add_arr(query, 'RADARR')


def add_to_sonarr(query):
    return add_arr(query, 'SONARR')
