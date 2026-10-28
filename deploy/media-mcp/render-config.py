#!/usr/bin/env python3
import json
import os
import re
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
PLACEHOLDER = re.compile(r"\$\{([A-Z0-9_]+)\}")


def read_env(path):
    values = {}
    for line in path.read_text().splitlines():
        line = line.strip()
        if not line or line.startswith("#") or "=" not in line:
            continue
        key, value = line.split("=", 1)
        values[key.strip()] = value.strip().strip('"').strip("'")
    return values


def main():
    env_path = HERE / ".env"
    if not env_path.exists():
        sys.exit(f"missing {env_path}; copy .env.example and fill it in")
    values = read_env(env_path)

    template = (HERE / "config.template.json").read_text()
    missing = sorted({name for name in PLACEHOLDER.findall(template) if not values.get(name)})
    if missing:
        sys.exit("empty or missing in .env: " + ", ".join(missing))

    rendered = PLACEHOLDER.sub(lambda match: json.dumps(values[match.group(1)])[1:-1], template)
    json.loads(rendered)

    output = HERE / "config.json"
    descriptor = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o600)
    with os.fdopen(descriptor, "w") as handle:
        handle.write(rendered)
    os.chmod(output, 0o600)
    print(f"wrote {output}")


if __name__ == "__main__":
    main()
