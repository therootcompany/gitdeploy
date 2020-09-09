# cdadmin

## Features

- Create a webserver that has an API and an admin interface
- Passwordless authentication using email, sign-in available for users who are on a list
- Handles webhooks that rebuild staging
- Gives admin a button to rebuild staging
- Gives users feedback on the staging rebuild status
- Handles webhooks that rebuild production
- Gives admin a button to rebuild production
- Gives users feedback on the production rebuild status
- Gives users a button to rebase production on staging (did I say that correctly?)

## Email

Need to support at least the following for sending email.

- SMTP
- Amazon SES
- Mailgun
