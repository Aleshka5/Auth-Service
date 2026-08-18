# This is Go based web service with Google OAuth2 + JWT

## Backend
We use best practicies for Go development.

Must implement Role System for this service and for any other.
This service Roles:
1) Admin
2) User
(Not Hardcode it, make it extandable)

White list verified redirect servicies. When any other web page redirect here to get auth we must check returned_to filed using configured white list like: storage.filenkov.store, llm.filenkov.store, filenkov.store ect
{
    "storage.filenkov.store": (name, storage_roles)
    "resume.filenkov.store": (name, resume_roles)
    "llm.filenkov.store": (name, llm_roles)
    "filenkov.store": (google_email, name, roles)
}
key - valid url
() - list of available db user fields to get

Must implement Rate Limit Gate by ip

### Logging
Service must log every important step with json capable format to use loki in the future.

### DB
Users Table like
id | google_email | name | roles | storage_roles | ...

use Enums for every roles column

### Redis Cache
For fast Auth purposes, Rate Limitting, storring white list

### API
Make all oauth standart API Endpoints
+ This service will be used as auth service for other servicies so it must implement validate other connections for example.


## Frontend
Prefere Vite framework
For prod version it must be builded static FS site.

1) Auth page (filenkov.store/auth) . Only google OAuth2 authentication with user info request then.
2) Main page (filenkov.store). Available without auth, but Have some UI flags for auth users - SignIn button for unknown user, Show Name for known user.
Page with some links and About section. One page with header and footer.
Links should place there: llm.filenkov.store, storage.filenkov.store and filenkov.store/hermes/ - only for admin
3) Admin page (filenkov.store/admin). Admin authenticated user only available. Contains List of registered users with their roles.
Admin can change role and block users.
User name | Email | Block Button | Status (Last activity date time or Blocked)
roles []
storage roles []
...