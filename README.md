# [gitdeploy](https://git.rootprojects.org/root/gitdeploy)

**gitdeploy** is an app for continuous deployment of static websites.

## Features

**gitdeploy** is intended for use with static websites that are generated after
changes are pushed to a Git repository. This works with sites that are being
edited in code and tracked in Git. Sites that have their content managed with a
headless CMS that pushes to Git are also very well-suited.

**gitdeploy** supports verified webhooks from Github, Bitbucket, and Gitea.

**gitdeploy** is written in Go. This means that it's a standalone binary
available on all major operating systems and architectures. It provides an API
with endpoints that handle webhooks, allow for initiation of builds, and getting
the status of builds and build jobs.

**gitdeploy** comes with a simple interface. The interface be disabled if you
don't want to use it.

## Usage

```bash
gitdeploy run --listen :3000 --serve-path ./public_overrides --exec ./scripts/
```

## Install

You can download `gitdeploy` from the Github Releases API and place it in your PATH,
or install it with Webi:

**Mac**, **Linux**:

```bash
curl -sS https://webinstall.dev/gitdeploy | bash
```

**Windows 10**:

```bash
curl -A MS https://webinstall.dev/gitdeploy | powershell
```

## Setup with Deploy Scripts

Start by copying from `examples/` to `scripts/`.

```bash
rsync -av examples/ scripts/
```

```txt
scripts/
├── deploy.sh
├── git.example.com/org/go-project/deploy.sh
├── git.example.com/org/node-project/deploy.sh
├── git.example.com/org/mirror-project/deploy.sh
└── promote.sh
```

The default `deploy.sh` is sensible -
if another `deploy.sh` exists in a directory with the same repo name
as an incoming webhook, it runs it.

The example deploy scripts are a good start, but you'll probably
need to update them to suit your build process for your project.

### Git Info

These ENVs are set before each script is run:

```bash
GIT_DEPLOY_JOB_ID=xxxxxx
GIT_REF_NAME=master
GIT_REF_TYPE=branch
GIT_REPO_OWNER=example
GIT_REPO_NAME=example
GIT_CLONE_URL=https://github.com/example/example
```

## API

```txt
GET  /api/admin/jobs

    {
      "success": true,
      "jobs": [
        {
          "job_id":     "xxxx",
          "created_at": "2020-01-01T00:00:00Z",
          "ref":        "0000000",
        }
      ]
    }

POST /api/admin/jobs
  { "job_id": "xxxx", "kill": true }

  { "success": true }

# note: see --help for how to use --promotions
POST /api/admin/promote
  { "clone_url": "https://...", "ref_name": "development" }

  { "success": true, "promote_to": "staging" }

# note: each webhook is different, but the result is to run a deploy.sh
POST /api/admin/webhooks/{github,gitea,bitbucket}
```

## Build

**Frontend**:

```bash
pushd html/
  npm ci
  scripts/build
popd
```

**API**:

With [GoReleaser](https://webinstall.dev/goreleaser):

```bash
goreleaser --snapshot --skip-publish --rm-dist
```

With [Golang](https://webinstall.dev/golang):

```bash
export GOFLAGS="-mod=vendor"

go run -mod=vendor git.rootprojects.org/root/go-gitver/v2
go generate -mod=vendor ./...
go build -mod=vendor .
```

You can use build tags to remove providers from the build:

```bash
go build -mod=vendor -tags nobitbucket,nogithub .
```

Supported tags are:

- nogithub
- nobitbucket

## Run as a System Service

```bash
sudo env PATH="$PATH" \
  serviceman add --name gitdeploy --system \
    --username app -path "$PATH" -- \
    gitdeploy run --exec ./scripts/
```

## Add Webhooks

To add a webhook you'll first need a secret

**with node.js**:

```js
crypto.randomBytes(16).toString("hex");
```

Then you'll need to set up the webhook in your platform of choice.

### Github

New Webhook: `https://github.com/YOUR_ORG/YOUR_REPO/settings/hooks/new`

```txt
Payload URL: https://YOUR_DOMAIN/api/webhooks/github
Content-Type: application/json
Secret: YOUR_SECRET
Which events would you like to trigger this webhook?
Just the `push` event.
Active: ✅
```

### Bitbucket

Sometimes Bitbucket does not give you the option to specify the (`X-Hub-Signature`) `secret`,
so you'll have to append an `access_token` instead. Example:

```txt
Title: gitdeploy
URL: https://YOUR_DOMAIN/api/webhooks/bitbucket?access_token=YOUR_SECRET
Triggers: Repository push
```

## How to Generate a Base64 Secret

**in your browser**:

```js
(async function () {
  var rnd = new Uint8Array(16);
  await crypto.getRandomValues(rnd);
  var b64 = [].slice
    .apply(rnd)
    .map(function (ch) {
      return String.fromCharCode(ch);
    })
    .join("");
  var secret = btoa(b64)
    .replace(/\//g, "_")
    .replace(/\+/g, "-")
    .replace(/=/g, "");
  console.info(secret);
})();
```

**with node.js**:

```js
crypto
  .randomBytes(16)
  .toString("base64")
  .replace(/\+/g, "-")
  .replace(/\//g, "_")
  .replace(/=/g, "");
```

## License

Copyright 2020 The gitdeploy Authors

This Source Code Form is subject to the terms of the Mozilla Public \
License, v. 2.0. If a copy of the MPL was not distributed with this \
file, You can obtain one at https://mozilla.org/MPL/2.0/.
