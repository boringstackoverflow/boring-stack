.PHONY: build serve clean

# Render docs/issues/*.md → docs/issues/*.html, regenerate the archive
# index, and update docs/feed.xml. Commit the rendered output.
#
# Adding a new issue:
#   1. Create docs/issues/YYYY-MM-DD-some-slug.md with frontmatter
#      (title, date, summary, draft).
#   2. make build
#   3. git add docs/issues/*.{md,html} docs/feed.xml && git commit
#   4. git push — GitHub Pages serves it; api.boringstack.org's worker
#      polls the feed and auto-sends within ~5 min (unless draft: true).
build:
	cd tools && go run .

# Local preview at http://127.0.0.1:8000
serve:
	cd docs && python3 -m http.server 8000

clean:
	rm -f docs/issues/*.html docs/feed.xml
