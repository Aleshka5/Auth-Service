"""DashboardAuthProvider that verifies the Auth-Service access JWT.

The Auth Gateway already requires ``global_role=ADMIN`` before proxying
``/hermes/*``. It injects that request's access cookie as
``Authorization: Bearer <jwt>`` so Hermes's gated middleware can authenticate
without a second login page.

Configure on the Hermes side (secrets belong in ``.env``):

    HERMES_DASHBOARD_AUTH_JWT_SECRET=<same value as Auth-Service JWT_SECRET>
    HERMES_DASHBOARD_AUTH_JWT_ISSUER=https://filenkov.store
    HERMES_DASHBOARD_AUTH_JWT_AUDIENCE=filenkov.store

Optional: ``HERMES_DASHBOARD_AUTH_LOGIN_URL`` for the Hermes ``/login`` button
(defaults to ``{issuer}/auth``).
"""
from __future__ import annotations

import logging
import os
from typing import Optional
from urllib.parse import urljoin

from hermes_cli.dashboard_auth import (
    DashboardAuthProvider,
    InvalidCodeError,
    LoginStart,
    RefreshExpiredError,
    Session,
)

try:
    from .jwtutil import JWTError, verify_access_jwt
except ImportError:  # loaded as a loose plugin module, not a package
    import sys
    from pathlib import Path

    _plugin_dir = str(Path(__file__).resolve().parent)
    if _plugin_dir not in sys.path:
        sys.path.insert(0, _plugin_dir)
    from jwtutil import JWTError, verify_access_jwt

logger = logging.getLogger(__name__)

LAST_SKIP_REASON: str = ""


def _env(name: str, default: str = "") -> str:
    return os.environ.get(name, default).strip()


class AuthGatewayProvider(DashboardAuthProvider):
    """Session provider backed by the Auth-Service access JWT."""

    name = "auth-gateway"
    display_name = "Auth Gateway"
    supports_session = True
    supports_password = False
    supports_token = False

    def __init__(
        self,
        *,
        secret: bytes,
        issuer: str,
        audiences: tuple[str, ...],
        login_url: str,
        require_admin: bool = True,
    ) -> None:
        if len(secret) < 32:
            raise ValueError("JWT secret must be at least 32 bytes")
        if not issuer:
            raise ValueError("issuer is required")
        if not audiences:
            raise ValueError("at least one audience is required")
        self._secret = secret
        self._issuer = issuer
        self._audiences = audiences
        self._login_url = login_url
        self._require_admin = require_admin

    def start_login(self, *, redirect_uri: str) -> LoginStart:
        # Direct hits on Hermes /login (not through the gateway) bounce to
        # the hub Google sign-in. After login the user must open /hermes/
        # again — there is no OAuth callback into Hermes.
        _ = redirect_uri
        return LoginStart(redirect_url=self._login_url, cookie_payload={})

    def complete_login(
        self,
        *,
        code: str,
        state: str,
        code_verifier: str,
        redirect_uri: str,
    ) -> Session:
        raise InvalidCodeError("auth-gateway has no Hermes OAuth callback")

    def verify_session(self, *, access_token: str) -> Optional[Session]:
        try:
            claims = verify_access_jwt(
                access_token,
                secret=self._secret,
                issuer=self._issuer,
                audiences=self._audiences,
                require_admin=self._require_admin,
            )
        except JWTError:
            return None
        return Session(
            user_id=str(claims["sub"]),
            email=str(claims.get("email") or ""),
            display_name=str(claims.get("username") or claims.get("email") or ""),
            org_id="",
            provider=self.name,
            expires_at=int(claims["exp"]),
            access_token=access_token,
            refresh_token="",
        )

    def refresh_session(self, *, refresh_token: str) -> Session:
        # Access JWT refresh stays at the Auth Gateway. Hermes must not mint
        # a substitute session from an empty or foreign refresh token.
        raise RefreshExpiredError("auth-gateway does not refresh Hermes sessions")

    def revoke_session(self, *, refresh_token: str) -> None:
        _ = refresh_token
        return None


def register(ctx) -> None:
    """Register when JWT verification can fail closed with a real secret."""
    global LAST_SKIP_REASON
    LAST_SKIP_REASON = ""

    secret = _env("HERMES_DASHBOARD_AUTH_JWT_SECRET")
    issuer = _env("HERMES_DASHBOARD_AUTH_JWT_ISSUER", "https://filenkov.store")
    aud_raw = _env("HERMES_DASHBOARD_AUTH_JWT_AUDIENCE", "filenkov.store")
    audiences = tuple(a for a in (p.strip() for p in aud_raw.split(",")) if a)
    login_url = _env("HERMES_DASHBOARD_AUTH_LOGIN_URL") or urljoin(
        issuer.rstrip("/") + "/", "auth"
    )

    if not secret:
        LAST_SKIP_REASON = (
            "HERMES_DASHBOARD_AUTH_JWT_SECRET is not set. Copy Auth-Service "
            "JWT_SECRET into the Hermes environment so the dashboard can "
            "verify the Bearer token the gateway injects on /hermes/*."
        )
        logger.debug("dashboard-auth-gateway: %s", LAST_SKIP_REASON)
        return
    if len(secret.encode("utf-8")) < 32:
        LAST_SKIP_REASON = (
            "HERMES_DASHBOARD_AUTH_JWT_SECRET is shorter than 32 bytes; "
            "refusing to register (fail closed)."
        )
        logger.warning("dashboard-auth-gateway: %s", LAST_SKIP_REASON)
        return

    try:
        provider = AuthGatewayProvider(
            secret=secret.encode("utf-8"),
            issuer=issuer,
            audiences=audiences,
            login_url=login_url,
        )
    except ValueError as exc:
        LAST_SKIP_REASON = f"AuthGatewayProvider construction failed: {exc}"
        logger.warning("dashboard-auth-gateway: %s", LAST_SKIP_REASON)
        return

    ctx.register_dashboard_auth_provider(provider)
    logger.info(
        "dashboard-auth-gateway: registered (issuer=%s audiences=%s)",
        issuer,
        ",".join(audiences),
    )
