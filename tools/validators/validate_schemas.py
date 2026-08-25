#!/usr/bin/env python3
"""Validate Atelier schemas and positive fixtures with Draft 2020-12."""

from __future__ import annotations

import copy
import json
from pathlib import Path

from jsonschema import ValidationError
from jsonschema.validators import validator_for
from referencing import Registry, Resource


ROOT = Path(__file__).resolve().parents[2]
SCHEMA_ROOT = ROOT / "schemas" / "v1"
FIXTURE_ROOT = ROOT / "tests" / "fixtures" / "schemas" / "v1"


def load_json(path: Path) -> object:
    with path.open("r", encoding="utf-8") as source:
        return json.load(source)


def main() -> None:
    schemas: dict[str, dict[str, object]] = {}
    resources: list[tuple[str, Resource[dict[str, object]]]] = []
    for path in sorted(SCHEMA_ROOT.glob("*.schema.json")):
        schema = load_json(path)
        if not isinstance(schema, dict) or not isinstance(schema.get("$id"), str):
            raise SystemExit(f"schema lacks an object root or $id: {path}")
        validator_for(schema).check_schema(schema)
        schema_id = schema["$id"]
        schemas[path.name.removesuffix(".schema.json")] = schema
        resources.append((schema_id, Resource.from_contents(schema)))

    registry = Registry().with_resources(resources)
    validators = {
        name: validator_for(schema)(schema, registry=registry)
        for name, schema in schemas.items()
    }

    fixture_count = 0
    for path in sorted(FIXTURE_ROOT.glob("*.json")):
        schema_name = path.name.split(".", 1)[0]
        validator = validators.get(schema_name)
        if validator is None:
            raise SystemExit(f"fixture has no matching schema: {path}")
        validator.validate(load_json(path))
        fixture_count += 1

    initialize = load_json(FIXTURE_ROOT / "command-result.initialize.json")
    if not isinstance(initialize, dict):
        raise SystemExit("initialize fixture must be an object")
    initialize_validator = validators["command-result"]
    for label, path, invalid_value in (
        ("created revision", ("data", "revision"), 1),
        ("created mode", ("data", "mode"), "manual"),
        ("created Godot version", ("data", "engine", "requested_version"), "4.8.0-stable"),
    ):
        candidate = copy.deepcopy(initialize)
        target = candidate
        for key in path[:-1]:
            target = target[key]
        target[path[-1]] = invalid_value
        try:
            initialize_validator.validate(candidate)
        except ValidationError:
            continue
        raise SystemExit(f"schema accepted invalid {label}")

    print(f"Draft 2020-12 schema validation PASS: {len(schemas)} schemas, {fixture_count} fixtures, 3 negative assertions")


if __name__ == "__main__":
    main()
