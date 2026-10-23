import tempfile
import unittest
from pathlib import Path

from check_docs import Checker


class DocumentationChecks(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        (self.root / "docs").mkdir()

    def write(self, name, text):
        path = self.root / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(text, encoding="utf-8")
        return path

    def check(self, text):
        path = self.write("README.md", text)
        return Checker(self.root).check([path])

    def test_reference_links_and_formatted_headings(self):
        self.write("docs/a.md", "# Hello **world**\n")
        self.assertEqual([], self.check("[Example][ref]\n\n[ref]: docs/a.md#hello-world\n"))

    def test_missing_reference_destination(self):
        issues = self.check("[Example][ref]\n\n[ref]: docs/missing.md\n")
        self.assertIn("missing local target", issues[0].message)

    def test_missing_same_document_anchor(self):
        self.assertIn("missing heading", self.check("# Present\n[x](#absent)\n")[0].message)

    def test_missing_image(self):
        self.assertIn("missing local target", self.check("![art](missing.png)\n")[0].message)

    def test_percent_encoded_path_and_fragment(self):
        self.write("docs/a b.md", "# Café\n")
        self.assertEqual([], self.check("[x](docs/a%20b.md#caf%C3%A9)\n"))

    def test_duplicate_heading_and_slug_collision(self):
        self.write("docs/a.md", "# Example\n# Example\n# Example-1\n")
        self.assertEqual([], self.check("[x](docs/a.md#example-1-1)\n"))

    def test_unknown_fragment(self):
        self.write("docs/a.md", "# Example\n")
        self.assertIn("missing heading", self.check("[x](docs/a.md#wrong)\n")[0].message)

    def test_explicit_html_anchor(self):
        self.assertEqual([], self.check('<a id="custom"></a>\n[x](#custom)\n'))

    def test_html_link_and_image(self):
        self.assertEqual(2, len(self.check('<a href="missing.md">x</a><img src="missing.png">\n')))

    def test_links_inside_code_are_ignored(self):
        self.assertEqual([], self.check("~~~text\n[x](missing.md)\n~~~\n"))

    def test_external_urls_are_not_fetched(self):
        self.assertEqual([], self.check("[x](https://example.invalid/a#b)\n[x](mailto:a@example.invalid)\n"))

    def test_directory_link(self):
        self.assertEqual([], self.check("[x](docs/)\n"))

    def test_repo_escape(self):
        self.assertIn("escapes repository", self.check("[x](../outside.md)\n")[0].message)

    def test_json_error(self):
        self.assertIn("json example", self.check('~~~json\n{"a":}\n~~~\n')[0].message)

    def test_yaml_duplicate_key(self):
        self.assertIn("duplicate YAML key", self.check("~~~yaml\na: 1\na: 2\n~~~\n")[0].message)

    def test_yaml_compose_reset(self):
        self.assertEqual([], self.check("~~~yaml\nservices:\n  redis:\n    ports: !reset []\n~~~\n"))

    def test_toml_error(self):
        self.assertIn("toml example", self.check("~~~toml\nkey = [\n~~~\n")[0].message)

    def test_shell_is_only_syntax_checked(self):
        self.assertEqual([], self.check("~~~sh\nexit 99\n~~~\n"))

    def test_shell_error(self):
        self.assertIn("sh example", self.check("~~~sh\nif true; then\n~~~\n")[0].message)

    def test_python_error(self):
        self.assertIn("python example", self.check("~~~python\nif:\n~~~\n")[0].message)

    def test_unclosed_fence(self):
        self.assertIn("unclosed code fence", self.check("~~~json\n{}\n")[0].message)


if __name__ == "__main__":
    unittest.main()
