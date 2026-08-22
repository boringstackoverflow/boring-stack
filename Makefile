.PHONY: build cli test serve showcase showcase-images clean

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

# Pull the approved listings from BSB and rebuild docs/showcase.html.
#
#   make showcase VPS=root@your-vps
#
# Review the JSON diff before committing: that diff is the moderation gate —
# nothing on the showcase page is published any other way (D22 in the backend
# repo). Run `bsb showcase verify --all` on the VPS first if it has been a while.
showcase:
	@test -n "$(VPS)" || (echo "usage: make showcase VPS=user@host" && exit 1)
	ssh $(VPS) 'sudo -u bsb /home/bsb/app/bsb showcase export' > docs/showcase.json
	$(MAKE) showcase-images ARGS=--prune
	cd tools && go run .
	@echo "review docs/showcase.json, docs/showcase/*, docs/showcase.html, then commit"

# Fetch a preview image for each listing: the site's own og:image, or a headless
# Chrome screenshot of its homepage. Images are downloaded into docs/showcase/
# and committed, never hotlinked — a page that promises it records nothing about
# your visitors must not make their browsers call out to six other domains.
#
# Safe to re-run: existing images are skipped unless you pass --force.
# Listings whose site is unreachable are skipped and render a placeholder.
showcase-images:
	cd tools && go run ./images $(ARGS)

cli:
	go build -ldflags="-X main.version=$$(git rev-parse --short HEAD 2>/dev/null || echo dev)" -o boringstack ./cmd/boringstack

test:
	go test ./...
	cd tools && go test ./...
	bash -n docs/install.sh docs/add.sh

# Local preview at http://127.0.0.1:8000
serve:
	cd docs && python3 -m http.server 8000

clean:
	rm -f docs/issues/*.html docs/feed.xml docs/showcase.html boringstack
	@echo 'note: docs/showcase/ images are kept — remove by hand if you mean it'
