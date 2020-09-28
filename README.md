# [git-deploy](https://git.ryanburnette.com/ryanburnette/git-deploy)

**git-deploy** is an app for handling continuous deployment of static websites.

## Build

```bash
pushd html/
npm install
./scripts/development
popd
```

```bash
go mod tidy
go mod vendor
go generate -mod=vendor ./...
go build -mod=vendor .
```

```bash
./git-deploy run --listen :3000 --serve-path ./overrides
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

## TODO

**git-deploy** is intended for use with static websites that are generated after
changes are pushed to a Git repository. This works with sites that are being
edited in code and tracked in Git. Sites that have their content managed with a
headless CMS that pushes to Git are also very well-suited.

**git-deploy** supports verified webhooks from Github, Bitbucket, and Gitea.

**git-deploy** is written in Go. This means that it's a standalone binary
available on all major operating systems and architectures. It provides an API
with endpoints that handle webhooks, allow for initiation of builds, and getting
the status of builds and build jobs.

**git-deploy** comes with a simple interface. The interface be disabled if you
don't want to use it.

**git-deploy** also comes with basic authentication via integration with
[Pocket ID](https://pocketid.app). Authentication can also be disabled if you
don't want to use it. The built-in interface requires the built-in
authentication.

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

Copyright 2020. All rights reserved.
