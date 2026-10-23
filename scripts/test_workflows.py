import unittest
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]


def workflow(name):
    return yaml.load((ROOT / ".github" / "workflows" / name).read_text(), Loader=yaml.BaseLoader)


class WorkflowPolicy(unittest.TestCase):
    def test_every_push_and_pr_is_triggered_without_filters(self):
        events = workflow("docker-publish.yml")["on"]
        self.assertIn("push", events)
        self.assertIn("pull_request", events)
        self.assertFalse(events["push"])
        self.assertFalse(events["pull_request"])

    def test_publication_waits_for_docs_and_go_checks(self):
        jobs = workflow("docker-publish.yml")["jobs"]
        self.assertEqual({"documentation", "go-checks"}, set(jobs["build-and-push"]["needs"]))
        self.assertEqual("./.github/workflows/documentation.yml", jobs["documentation"]["uses"])
        self.assertEqual("./.github/workflows/test.yml", jobs["go-checks"]["uses"])

    def test_both_runtime_targets_and_architectures(self):
        job = workflow("docker-publish.yml")["jobs"]["build-and-push"]
        self.assertEqual({"final", "with-python"}, {v["target"] for v in job["strategy"]["matrix"]["include"]})
        build = next(s for s in job["steps"] if s.get("id") == "build")
        self.assertEqual("linux/amd64,linux/arm64", build["with"]["platforms"])
        self.assertIn("pull_request", build["with"]["push"])

    def test_prs_cannot_promote_and_login_is_conditional(self):
        jobs = workflow("docker-publish.yml")["jobs"]
        self.assertEqual("github.event_name != 'pull_request'", jobs["promote"]["if"])
        login = next(s for s in jobs["build-and-push"]["steps"] if "docker/login-action" in s.get("uses", ""))
        self.assertEqual("github.event_name != 'pull_request'", login["if"])

    def test_builds_are_not_canceled_by_ref_concurrency(self):
        spec = workflow("docker-publish.yml")
        self.assertNotIn("concurrency", spec)
        self.assertNotIn("concurrency", spec["jobs"]["build-and-push"])
        self.assertEqual("false", spec["jobs"]["promote"]["concurrency"]["cancel-in-progress"])

    def test_go_checks_include_required_commands(self):
        steps = workflow("test.yml")["jobs"]["test"]["steps"]
        runs = {s.get("run") for s in steps}
        self.assertTrue({"go build ./...", "go vet ./...", "go test -race ./..."}.issubset(runs))

    def test_docs_workflow_checks_itself_and_the_repository(self):
        steps = workflow("documentation.yml")["jobs"]["documentation"]["steps"]
        runs = "\n".join(s.get("run", "") for s in steps)
        self.assertIn("unittest discover", runs)
        self.assertIn("check_docs.py", runs)
        self.assertIn("test_image_tags.cjs", runs)

    def test_documentation_permissions_are_read_only(self):
        self.assertEqual({"contents": "read"}, workflow("documentation.yml")["permissions"])


if __name__ == "__main__":
    unittest.main()
