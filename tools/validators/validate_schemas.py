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


def expect_invalid(validator: object, candidate: object, label: str) -> None:
    try:
        validator.validate(candidate)
    except ValidationError:
        return
    raise SystemExit(f"schema accepted invalid {label}")


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
        expect_invalid(initialize_validator, candidate, label)

    headless_intent = load_json(FIXTURE_ROOT / "run-intent.headless.json")
    if not isinstance(headless_intent, dict):
        raise SystemExit("headless run-intent fixture must be an object")
    invalid_intent = copy.deepcopy(headless_intent)
    invalid_intent["declared_external_writes"] = []
    expect_invalid(
        validators["run-intent"],
        invalid_intent,
        "authorized headless intent without its external write declaration",
    )

    invalid_build_intent = copy.deepcopy(headless_intent)
    invalid_build_intent["command"]["name"] = "build"
    expect_invalid(
        validators["run-intent"],
        invalid_build_intent,
        "non-headless command with a Godot user-data declaration",
    )

    legacy_intent = load_json(FIXTURE_ROOT / "run-intent.legacy-baseline.json")
    if not isinstance(legacy_intent, dict) or "declared_external_writes" in legacy_intent:
        raise SystemExit("legacy baseline intent must exercise the optional external-write field")
    validators["run-intent"].validate(legacy_intent)

    headless_result = load_json(FIXTURE_ROOT / "command-result.headless.json")
    headless_report = load_json(FIXTURE_ROOT / "validation-report.headless.json")
    if not isinstance(headless_result, dict) or not isinstance(headless_report, dict):
        raise SystemExit("headless result and validation report fixtures must be objects")
    result_data = headless_result.get("data")
    report_checks = headless_report.get("checks")
    if not isinstance(result_data, dict) or not isinstance(report_checks, list):
        raise SystemExit("headless fixtures lack structured data or checks")
    if headless_result["command"]["arguments"].get("headless") is not True:
        raise SystemExit("headless command fixture is not marked headless")
    if result_data.get("scope") != headless_report.get("scope"):
        raise SystemExit("headless command result and report scopes differ")
    if headless_result.get("outcome") != headless_report.get("outcome"):
        raise SystemExit("headless command result and report outcomes differ")
    if result_data.get("check_count") != len(report_checks):
        raise SystemExit("headless command result check_count differs from its report")

    invalid_headless_result = copy.deepcopy(headless_result)
    invalid_headless_result["data"]["scope"] = "baseline"
    expect_invalid(
        validators["command-result"],
        invalid_headless_result,
        "headless command result with baseline data scope",
    )

    print(f"Draft 2020-12 schema validation PASS: {len(schemas)} schemas, {fixture_count} fixtures, 6 negative assertions, headless cross-fixture semantics")


if __name__ == "__main__":
    main()
