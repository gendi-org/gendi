.PHONY: default clean serve build

# Must match static/CNAME. Override to build for the GitHub Pages project path:
# make build BASE_URL=https://gendi-org.github.io/gendi/
BASE_URL ?= https://gendi.dev/

default: serve

clean:
	rm -rf public/

serve: clean
	hugo server --disableFastRender --buildDrafts --enableGitInfo

# Used by .github/workflows/docs.yml — keep the two in step.
build: clean
	HUGO_ENVIRONMENT=production \
	HUGO_ENV=production \
	hugo \
		--gc --minify \
		--baseURL "$(BASE_URL)"
