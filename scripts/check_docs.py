#!/usr/bin/env python3
"""Validate first-party documentation without executing examples or fetching URLs."""
from __future__ import annotations

import argparse
from dataclasses import dataclass
from html.parser import HTMLParser
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tomllib
from urllib.parse import unquote, urlsplit

from markdown_it import MarkdownIt
import yaml


class ExampleLoader(yaml.SafeLoader):
    pass


def mapping(loader, node, deep=False):
    keys = set()
    for key, _ in node.value:
        if key.tag == "tag:yaml.org,2002:merge":
            continue
        value = loader.construct_object(key, deep=deep)
        if value in keys:
            raise ValueError("duplicate YAML key: " + str(value))
        keys.add(value)
    loader.flatten_mapping(node)
    return yaml.SafeLoader.construct_mapping(loader, node, deep=deep)


ExampleLoader.add_constructor("tag:yaml.org,2002:map", mapping)
ExampleLoader.add_constructor("!reset", lambda loader, node: loader.construct_sequence(node))


class HTMLReferences(HTMLParser):
    def __init__(self):
        super().__init__()
        self.links = []
        self.anchors = set()

    def handle_starttag(self, tag, attrs):
        values = dict(attrs)
        for key in ("id", "name" if tag == "a" else "id"):
            if values.get(key):
                self.anchors.add(values[key])
        if tag == "a" and values.get("href"):
            self.links.append(values["href"])
        if tag in ("img", "source") and values.get("src"):
            self.links.append(values["src"])


@dataclass(frozen=True)
class Issue:
    path: Path
    line: int
    message: str


def heading_text(children):
    result = []
    for token in children or []:
        if token.type in ("text", "code_inline", "image"):
            result.append(token.content)
        elif token.type in ("softbreak", "hardbreak"):
            result.append(" ")
    return "".join(result)


def slug(text):
    return re.sub(r"[^\w\- ]", "", text.lower()).replace(" ", "-")


class Checker:
    def __init__(self, root, examples=True):
        self.root = Path(root).resolve()
        self.examples = examples
        self.parser = MarkdownIt("commonmark").enable(["table", "strikethrough"])
        self.documents = {}
        self.issues = []
        self.link_count = 0
        self.example_count = 0

    def issue(self, path, line, message):
        self.issues.append(Issue(path, line, message))

    def document(self, path):
        path = Path(path).resolve()
        if path in self.documents:
            return self.documents[path]
        source = path.read_text(encoding="utf-8")
        tokens = self.parser.parse(source)
        anchors = set()
        for index, token in enumerate(tokens):
            if token.type == "heading_open":
                text = heading_text(tokens[index + 1].children)
                base = slug(text)
                anchor = base
                suffix = 0
                while anchor in anchors:
                    suffix += 1
                    anchor = base + "-" + str(suffix)
                anchors.add(anchor)
            elif token.type in ("html_block", "inline"):
                fragments = [token.content] if token.type == "html_block" else [
                    child.content for child in token.children or [] if child.type == "html_inline"
                ]
                for fragment in fragments:
                    html = HTMLReferences()
                    html.feed(fragment)
                    anchors.update(html.anchors)
        result = (source, tokens, anchors)
        self.documents[path] = result
        return result

    def link(self, path, line, href):
        if not href:
            self.issue(path, line, "empty link destination")
            return
        try:
            parsed = urlsplit(href)
        except ValueError as error:
            self.issue(path, line, "invalid link: " + str(error))
            return
        if parsed.scheme or parsed.netloc:
            return
        self.link_count += 1
        if not parsed.path:
            target = path
        elif parsed.path.startswith("/"):
            target = (self.root / unquote(parsed.path).lstrip("/")).resolve()
        else:
            target = (path.parent / unquote(parsed.path)).resolve()
        if not target.is_relative_to(self.root):
            self.issue(path, line, "link escapes repository: " + href)
        elif not target.exists():
            self.issue(path, line, "missing local target: " + href)
        elif parsed.fragment and target.suffix.lower() == ".md" and target.is_file():
            try:
                anchors = self.document(target)[2]
            except (OSError, UnicodeError) as error:
                self.issue(path, line, "cannot read target: " + str(error))
                return
            if unquote(parsed.fragment) not in anchors:
                self.issue(path, line, "missing heading or anchor: " + href)

    def example(self, path, source, token):
        language = token.info.split()[0].lower() if token.info.strip() else ""
        line = token.map[0] + 1
        final_line = source.splitlines()[token.map[1] - 1]
        ending = r"^\s*(?:>\s*)*" + re.escape(token.markup[0]) + "{" + str(len(token.markup)) + r",}\s*$"
        if not re.match(ending, final_line):
            self.issue(path, line, "unclosed code fence")
            return
        if not self.examples or language not in {"json", "yaml", "yml", "toml", "sh", "bash", "shell", "python"}:
            return
        self.example_count += 1
        try:
            if language == "json":
                json.loads(token.content)
            elif language in {"yaml", "yml"}:
                yaml.load(token.content, Loader=ExampleLoader)
            elif language == "toml":
                tomllib.loads(token.content)
            elif language == "python":
                compile(token.content, str(path), "exec")
            else:
                result = subprocess.run(
                    ["bash", "-n"], input=token.content, text=True,
                    capture_output=True, timeout=10, check=False,
                )
                if result.returncode:
                    raise ValueError(result.stderr.strip())
        except (ValueError, SyntaxError, yaml.YAMLError, OSError, subprocess.TimeoutExpired) as error:
            self.issue(path, line, language + " example: " + str(error))

    def check(self, paths=None):
        if paths is None:
            paths = sorted(set(self.root.glob("*.md")) | set((self.root / "docs").rglob("*.md")))
        for path in paths:
            path = Path(path).resolve()
            try:
                source, tokens, _ = self.document(path)
            except (OSError, UnicodeError) as error:
                self.issue(path, 1, "cannot read document: " + str(error))
                continue
            for token in tokens:
                line = token.map[0] + 1 if token.map else 1
                if token.type == "fence":
                    self.example(path, source, token)
                elif token.type == "inline":
                    for child in token.children or []:
                        if child.type in ("link_open", "image"):
                            self.link(path, line, child.attrGet("href") if child.type == "link_open" else child.attrGet("src"))
                        elif child.type == "html_inline":
                            html = HTMLReferences()
                            html.feed(child.content)
                            for href in html.links:
                                self.link(path, line, href)
                elif token.type == "html_block":
                    html = HTMLReferences()
                    html.feed(token.content)
                    for href in html.links:
                        self.link(path, line, href)
        return self.issues


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--root", type=Path, default=Path(__file__).resolve().parents[1])
    args = parser.parse_args()
    checker = Checker(args.root)
    issues = checker.check()
    for issue in issues:
        name = str(issue.path.relative_to(checker.root))
        print(f"{name}:{issue.line}: {issue.message}", file=sys.stderr)
        if os.environ.get("GITHUB_ACTIONS") == "true":
            message = issue.message.replace("%", "%25").replace("\r", "%0D").replace("\n", "%0A")
            annotation_path = name.replace("%", "%25").replace(",", "%2C").replace("\r", "%0D").replace("\n", "%0A")
            print(f"::error file={annotation_path},line={issue.line}::{message}")
    print(f"Documentation: {len(checker.documents)} Markdown files parsed, "
          f"{checker.link_count} local links, {checker.example_count} examples, {len(issues)} errors.")
    return 1 if issues else 0


if __name__ == "__main__":
    sys.exit(main())
