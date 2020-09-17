# cdadmin

**cdadmin** is administration app for continuous deployment of static websites.

**cdadmin** is intended to be used with static websites that are generated after
changes are pushed to a Git repository. This works with sites that are being
edited in code and tracked in Git, as well as with headless CMSs that are
pushing changes via Git.

cdadmin starts a web server that provides an admin interface as well as an API.

No authentication is provided, so you'll want to reverse proxy through something
that protects the endpoints.

## Admin

The main page of the admin show the staging and production rebuild status. It
allows the user to queue a rebuild of the staging and production environments.
It also allows the user to merge staging into production.

- Handles webhooks that rebuild staging
- Gives admin a button to rebuild staging
- Gives users feedback on the staging rebuild status
- Handles webhooks that rebuild production
- Gives admin a button to rebuild production
- Gives users feedback on the production rebuild status
- Gives users a button to rebase production on staging (did I say that
  correctly?)

## Email

Need to support at least the following for sending email.

- SMTP
- Amazon SES
- Mailgun
