#!/usr/bin/env python3
"""Validate Atelier schemas and positive fixtures with Draft 2020-12."""

from __future__ import annotations

import copy
import hashlib
import json
from pathlib import Path

from jsonschema import ValidationError
from jsonschema.validators import validator_for
from referencing import Registry, Resource


ROOT = Path(__file__).resolve().parents[2]
SCHEMA_ROOT = ROOT / "schemas" / "v1"
FIXTURE_ROOT = ROOT / "tests" / "fixtures" / "schemas" / "v1"
STARTER_EVIDENCE_ROOT = ROOT / "docs" / "validation" / "evidence" / "m1-vertical-slice-2026-08-28"
PROFILE_CATALOG = ROOT / "plugin" / "codex-game-atelier" / "skills" / "develop-godot-game" / "references" / "capability-profiles.json"
COLLABORATION_EVIDENCE_ROOT = ROOT / "docs" / "validation" / "evidence" / "m2-native-collaboration-2026-08-28"


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

    profile_catalog = load_json(PROFILE_CATALOG)
    validators["capability-profile-catalog"].validate(profile_catalog)
    if profile_catalog != load_json(FIXTURE_ROOT / "capability-profile-catalog.default.json"):
        raise SystemExit("distributed capability profile catalog differs from its contract fixture")

    persisted_evidence = {
        "initialize-first.json": "command-result",
        "initialize-repeat.json": "command-result",
        "project-state.json": "project-state",
        "validate-result.json": "command-result",
        "validation-report.json": "validation-report",
        "test-result.json": "command-result",
        "test-report.json": "test-report",
        "build-result.json": "command-result",
        "build-export-artifact.json": "export-artifact",
        "export-result.json": "command-result",
        "release-export-artifact.json": "export-artifact",
    }
    for name, schema_name in persisted_evidence.items():
        validators[schema_name].validate(load_json(STARTER_EVIDENCE_ROOT / name))

    persisted_collaboration = {
        "task-implementation.json": "task",
        "handoff-to-implementation.json": "handoff",
        "task-audit.json": "task",
        "handoff-to-audit.json": "handoff",
        "evidence-audit-round-1.json": "evidence",
        "task-repair.json": "task",
        "handoff-audit-to-repair.json": "handoff",
        "evidence-implementation-repair.json": "evidence",
        "task-reaudit.json": "task",
        "handoff-repair-to-reaudit.json": "handoff",
        "evidence-audit-round-2.json": "evidence",
        "handoff-reaudit-to-lead.json": "handoff",
        "task-verified.json": "task",
    }
    for name, schema_name in persisted_collaboration.items():
        validators[schema_name].validate(load_json(COLLABORATION_EVIDENCE_ROOT / name))
    for name in ("evidence-audit-round-1.json", "evidence-implementation-repair.json", "evidence-audit-round-2.json"):
        record = load_json(COLLABORATION_EVIDENCE_ROOT / name)
        if not isinstance(record, dict) or not isinstance(record.get("path"), str):
            raise SystemExit(f"collaboration evidence path is invalid: {name}")
        payload = ROOT / record["path"]
        content = payload.read_bytes()
        if record.get("byte_size") != len(content) or record.get("sha256") != hashlib.sha256(content).hexdigest():
            raise SystemExit(f"collaboration evidence digest is invalid: {name}")

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

    test_intent = load_json(FIXTURE_ROOT / "run-intent.test.json")
    test_result = load_json(FIXTURE_ROOT / "command-result.test.json")
    test_report = load_json(FIXTURE_ROOT / "test-report.gdscript.json")
    if not isinstance(test_intent, dict) or not isinstance(test_result, dict) or not isinstance(test_report, dict):
        raise SystemExit("test fixtures must be objects")
    result_test_data = test_result.get("data")
    report_tests = test_report.get("tests")
    if not isinstance(result_test_data, dict) or not isinstance(report_tests, list):
        raise SystemExit("test fixtures lack structured counts or cases")
    if result_test_data.get("test_count") != len(report_tests):
        raise SystemExit("test command result count differs from its report")
    if result_test_data.get("passed_count") != sum(case.get("outcome") == "PASS" for case in report_tests):
        raise SystemExit("test command result passed_count differs from its report")
    if test_result.get("outcome") != test_report.get("outcome"):
        raise SystemExit("test command result and report outcomes differ")

    invalid_fail_without_engine = copy.deepcopy(test_report)
    invalid_fail_without_engine["outcome"] = "FAIL"
    invalid_fail_without_engine.pop("engine_version")
    invalid_fail_without_engine["tests"][0]["outcome"] = "FAIL"
    expect_invalid(
        validators["test-report"],
        invalid_fail_without_engine,
        "nonempty failing test report without an engine version",
    )
    invalid_fail_without_failed_case = copy.deepcopy(test_report)
    invalid_fail_without_failed_case["outcome"] = "FAIL"
    expect_invalid(
        validators["test-report"],
        invalid_fail_without_failed_case,
        "failing test report without a failed case",
    )
    for summary, label in (("\u0001", "control-character test summary"), ("   ", "whitespace-only test summary")):
        invalid_test_summary = copy.deepcopy(test_report)
        invalid_test_summary["tests"][0]["summary"] = summary
        expect_invalid(validators["test-report"], invalid_test_summary, label)

    invalid_test_runner = copy.deepcopy(test_result)
    invalid_test_runner["command"]["arguments"]["test_runner"] = "res://tests/custom.gd"
    expect_invalid(
        validators["command-result"],
        invalid_test_runner,
        "test command with a caller-selected runner",
    )
    invalid_test_counts = copy.deepcopy(test_result)
    invalid_test_counts["data"]["failed_count"] = 1
    expect_invalid(
        validators["command-result"],
        invalid_test_counts,
        "passing test command with a nonzero failed count",
    )
    invalid_test_intent = copy.deepcopy(test_intent)
    invalid_test_intent["declared_external_writes"] = []
    expect_invalid(
        validators["run-intent"],
        invalid_test_intent,
        "authorized test intent without its external write declaration",
    )

    export_intent = load_json(FIXTURE_ROOT / "run-intent.export.json")
    export_result = load_json(FIXTURE_ROOT / "command-result.export.json")
    export_manifest = load_json(FIXTURE_ROOT / "export-artifact.macos.json")
    if not isinstance(export_intent, dict) or not isinstance(export_result, dict) or not isinstance(export_manifest, dict):
        raise SystemExit("export fixtures must be objects")
    export_data = export_result.get("data")
    export_artifact = export_manifest.get("artifact")
    if not isinstance(export_data, dict) or not isinstance(export_artifact, dict):
        raise SystemExit("export fixtures lack result data or artifact metadata")
    for field in ("target", "profile", "preset", "engine_version"):
        if export_data.get(field) != export_manifest.get(field):
            raise SystemExit(f"export result and manifest differ on {field}")
    if export_result.get("outcome") != export_manifest.get("outcome"):
        raise SystemExit("export result and manifest outcomes differ")
    expected_artifact = (
        f".gameatelier/artifacts/{export_result['run_id']}/"
        f"game-{export_data['profile']}.zip"
    )
    if export_artifact.get("path") != expected_artifact:
        raise SystemExit("export artifact path does not bind its run and profile")
    expected_writes = [
        f".gameatelier/runs/{export_result['run_id']}",
        f".gameatelier/artifacts/{export_result['run_id']}",
    ]
    if export_intent.get("declared_write_paths") != expected_writes:
        raise SystemExit("export intent does not declare its exact run and artifact roots")

    invalid_export_preset = copy.deepcopy(export_result)
    invalid_export_preset["command"]["arguments"]["preset"] = "Custom"
    expect_invalid(validators["command-result"], invalid_export_preset, "export result with a custom preset")
    invalid_export_count = copy.deepcopy(export_result)
    invalid_export_count["data"]["artifact_count"] = 0
    expect_invalid(validators["command-result"], invalid_export_count, "passing export without one artifact")
    invalid_export_external = copy.deepcopy(export_intent)
    invalid_export_external["declared_external_writes"] = []
    expect_invalid(validators["run-intent"], invalid_export_external, "authorized export without external write declaration")
    invalid_export_writes = copy.deepcopy(export_intent)
    invalid_export_writes["declared_write_paths"].pop()
    expect_invalid(validators["run-intent"], invalid_export_writes, "export intent without artifact write root")
    invalid_public_artifact = copy.deepcopy(export_manifest)
    invalid_public_artifact["artifact"]["public_distribution_ready"] = True
    expect_invalid(validators["export-artifact"], invalid_public_artifact, "unsigned artifact marked distribution ready")
    invalid_failed_artifact = copy.deepcopy(export_manifest)
    invalid_failed_artifact["outcome"] = "FAIL"
    expect_invalid(validators["export-artifact"], invalid_failed_artifact, "failed export containing an artifact")
    invalid_missing_smoke = copy.deepcopy(export_manifest)
    invalid_missing_smoke["artifact"].pop("target_smoke")
    expect_invalid(validators["export-artifact"], invalid_missing_smoke, "passing export without target smoke")

    logs_result = load_json(FIXTURE_ROOT / "command-result.logs.json")
    if not isinstance(logs_result, dict):
        raise SystemExit("logs fixture must be an object")
    logs_data = logs_result.get("data")
    if not isinstance(logs_data, dict) or not isinstance(logs_data.get("events"), list):
        raise SystemExit("logs fixture lacks structured events")
    invalid_logs_evidence = copy.deepcopy(logs_result)
    invalid_logs_evidence["evidence"] = [{"id": "unexpected", "path": ".gameatelier/unexpected.json"}]
    expect_invalid(validators["command-result"], invalid_logs_evidence, "read-only logs result with evidence")
    invalid_logs_project = copy.deepcopy(logs_result)
    invalid_logs_project["command"]["arguments"]["project"] = "/private/project"
    expect_invalid(validators["command-result"], invalid_logs_project, "logs result with an absolute project argument")
    invalid_logs_message = copy.deepcopy(logs_result)
    invalid_logs_message["data"]["events"][0]["message"] = "raw text"
    expect_invalid(validators["command-result"], invalid_logs_message, "logs event with free text")
    invalid_logs_empty = copy.deepcopy(logs_result)
    invalid_logs_empty["data"]["events"] = []
    expect_invalid(validators["command-result"], invalid_logs_empty, "passing logs result without events")
    invalid_logs_event_kind = copy.deepcopy(logs_result)
    invalid_logs_event_kind["data"]["events"][0]["kind"] = "check"
    expect_invalid(validators["command-result"], invalid_logs_event_kind, "test report event with check kind")
    invalid_logs_level = copy.deepcopy(logs_result)
    invalid_logs_level["data"]["events"][0]["level"] = "ERROR"
    expect_invalid(validators["command-result"], invalid_logs_level, "passing logs event with error level")
    invalid_logs_evidence_kind = copy.deepcopy(logs_result)
    invalid_logs_evidence_kind["data"]["evidence_kind"] = "validation-report"
    expect_invalid(validators["command-result"], invalid_logs_evidence_kind, "test source with validation evidence kind")
    invalid_logs_final_outcome = copy.deepcopy(logs_result)
    invalid_logs_final_outcome["data"]["events"][-1]["outcome"] = "FAIL"
    invalid_logs_final_outcome["data"]["events"][-1]["level"] = "ERROR"
    expect_invalid(validators["command-result"], invalid_logs_final_outcome, "passing source with failing final event")
    invalid_logs_source_exit = copy.deepcopy(logs_result)
    invalid_logs_source_exit["data"]["source_exit_code"] = 5
    expect_invalid(validators["command-result"], invalid_logs_source_exit, "passing source with nonzero exit")

    clean_result = load_json(FIXTURE_ROOT / "command-result.clean.json")
    if not isinstance(clean_result, dict):
        raise SystemExit("clean result fixture must be an object")
    future_schema_clean = copy.deepcopy(clean_result)
    future_schema_clean["data"]["protected"][0]["reason"] = "SCHEMA_UNSUPPORTED"
    validators["command-result"].validate(future_schema_clean)
    invalid_clean_reason = copy.deepcopy(clean_result)
    invalid_clean_reason["data"]["candidates"][0]["reason"] = "INTENT_AND_RESULT_MISSING"
    expect_invalid(
        validators["command-result"],
        invalid_clean_reason,
        "incomplete clean candidate with orphan reason",
    )
    invalid_partial_scan = copy.deepcopy(clean_result)
    invalid_partial_scan["data"]["scanned"] = False
    expect_invalid(
        validators["command-result"],
        invalid_partial_scan,
        "failed clean scan with partial decisions",
    )

    print(
        f"Draft 2020-12 schema validation PASS: {len(schemas)} schemas, "
        f"{fixture_count} fixtures, {len(persisted_evidence)} persisted Starter Template records, "
        f"{len(persisted_collaboration)} persisted collaboration records, "
        "31 negative assertions, headless, test, export, and logs cross-fixture semantics"
    )


if __name__ == "__main__":
    main()
