from pathlib import Path
import re
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = ROOT / ".github" / "workflows" / "ci.yml"


class CIWorkflowTests(unittest.TestCase):
    def test_minimal_macos_workflow_is_bounded_and_read_only(self) -> None:
        text = WORKFLOW.read_text(encoding="utf-8")
        document = yaml.load(text, Loader=yaml.BaseLoader)
        self.assertIsInstance(document, dict)
        self.assertEqual(document["permissions"], {"contents": "read"})
        self.assertEqual(set(document["jobs"]), {"verify-macos-arm64"})
        job = document["jobs"]["verify-macos-arm64"]
        self.assertEqual(job["runs-on"], "macos-15")
        self.assertEqual(job["timeout-minutes"], "25")
        self.assertNotIn("pull_request_target", document["on"])
        self.assertNotIn("write", text.lower())
        self.assertIn("persist-credentials: false", text)

    def test_external_actions_are_pinned_and_required_checks_are_present(self) -> None:
        text = WORKFLOW.read_text(encoding="utf-8")
        action_refs = re.findall(r"^\s*uses:\s*([^\s#]+)", text, flags=re.MULTILINE)
        self.assertEqual(len(action_refs), 3)
        for reference in action_refs:
            self.assertRegex(reference, r"^[a-z0-9-]+/[a-z0-9-]+@[a-f0-9]{40}$")
        for required in (
            "go-version: 1.24.0",
            "go vet ./...",
            "go test -count=1 ./...",
            "GOOS=linux GOARCH=amd64 go build",
            "GOOS=windows GOARCH=amd64 go build",
            "validate_schemas.py",
            "unittest discover",
            "./cmd/codex-game-atelier",
            "./cmd/codex-game-atelier-runner",
            'test "$runner_exit" -eq 125',
        ):
            self.assertIn(required, text)


if __name__ == "__main__":
    unittest.main()
