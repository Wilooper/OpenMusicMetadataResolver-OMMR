# Automatic container builds and publishing

[Hosting](README.md) · [Docker usage](docker.md) · [Documentation checks](../guides/documentation-checks.md)

The Validate and publish Docker images workflow runs on every branch/tag push,
every pull request and manual dispatch. There are no source-path filters, so a
documentation-only commit also updates the image's bundled docs.

## Pipeline

1. Run documentation-checker tests, image-tag policy tests, and documentation validation.
2. Run Go build, vet and the full race-test suite.
3. After both jobs pass, build the Go-only and optional Python targets for
   linux/amd64 and linux/arm64.
4. Publish commit images to GitHub Container Registry on pushes/manual runs.
5. Advance mutable aliases only while that commit is still the current branch/tag.

Pull requests build both variants/architectures but do not log in to GHCR or publish.
The workflow uses GitHub's built-in GITHUB_TOKEN with package-write permission;
no separate registry password or provider API key is required. Repository/organization
Actions policies and the existing GHCR package must allow the publishing repository.

## Image names

For this repository the registry image is ghcr.io/wilooper/ommr.
Replace FULL_COMMIT_SHA below with all 40 characters:

| Tag | Contents / update rule |
| --- | --- |
| sha-FULL_COMMIT_SHA | Go-only image for that exact pushed commit |
| sha-FULL_COMMIT_SHA-python | Same commit with the optional Python worker installed |
| latest | Current successful default-branch Go build only |
| latest-python | Current successful default-branch Python build only |
| branch-SLUG-REFHASH | Current successful build of that branch |
| branch-SLUG-REFHASH-python | That branch's Python variant |
| tag-SLUG-REFHASH | Exact pushed tag's Go build |
| 1.2.3 / 1.2.3-python | Exact stable version aliases for a v1.2.3 or 1.2.3 Git tag |

Branch/tag aliases include an eight-character hash of the full ref so names such
as feature/a and feature-a cannot collide. The Actions job summary lists the actual
image names and digests. Release tags do not overwrite default-branch latest.
Commit tags are commit-addressed; pin an image digest when registry-level immutability
is required. A manual rerun can rebuild the same commit from updated base images.

Every successful pushed commit keeps its commit image even if newer commits arrive.
Only the alias-promotion job is serialized per ref/variant; superseded pending
promotions may be skipped. Builds are not canceled by that concurrency group.
The promoter checks the live ref before updating aliases, preventing an older
slow build from replacing a newer branch image.

## Pull and run

~~~sh
docker pull ghcr.io/wilooper/ommr:latest
docker run --rm -p 127.0.0.1:8080:8080 ghcr.io/wilooper/ommr:latest
~~~

Use latest-python with the extractor environment/configuration described in the
[worker guide](../guides/python-worker.md). Installing the worker does not enable it
automatically. GHCR package visibility controls anonymous pulls; if a newly created
package is private, configure its visibility/access deliberately.

Building/publishing an image does not restart running containers or redeploy a host.
For an image-based Compose installation, pull the chosen tag and recreate the
service; for source builds, use the --build flow in the Docker guide. Pin a commit
tag or digest for reproducible deployment. Feature-branch pushes update branch tags;
they do not update default-branch latest before those changes are merged.

The Docker build context excludes common local secrets, credential/session files,
cache directories and virtual environments. Add any custom private paths to
.dockerignore before building; do not put credentials in the image.
