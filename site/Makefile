.PHONY: default clean serve build

# Swap this for the custom domain once the DNS record exists; nothing else in
# the site hardcodes the host.
BASE_URL ?= https://gendi-org.github.io/gendi/

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
