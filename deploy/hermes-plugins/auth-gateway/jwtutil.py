"""Stdlib HS256 access-JWT verify for the Auth-Service dashboard plugin."""

from __future__ import annotations

import base64
import hashlib
import hmac
import json
import time
import uuid
from typing import Any, Optional

ACCESS_TYP = "access"
ADMIN_ROLE = "ADMIN"
_LEEWAY_SECONDS = 5


class JWTError(ValueError):
    """Token is missing, malformed, expired, or not an admin access JWT."""


def b64url_decode(raw: str) -> bytes:
    pad = "=" * (-len(raw) % 4)
    return base64.urlsafe_b64decode(raw + pad)


def verify_access_jwt(
    token: str,
    *,
    secret: bytes,
    issuer: str,
    audiences: tuple[str, ...],
    now: Optional[int] = None,
    require_admin: bool = True,
) -> dict[str, Any]:
    """Return claims if ``token`` is a valid Auth-Service access JWT.

    Fail-closed: ``alg=none``, wrong HMAC, missing exp, wrong issuer/audience,
    wrong ``typ``, non-UUID ``sub``, or (when ``require_admin``) a non-ADMIN
    ``global_role`` all raise :class:`JWTError`.
    """
    token = (token or "").strip()
    if not token or not secret:
        raise JWTError("empty token or secret")

    parts = token.split(".")
    if len(parts) != 3:
        raise JWTError("malformed token")
    header_b64, payload_b64, sig_b64 = parts

    try:
        header = json.loads(b64url_decode(header_b64))
    except (ValueError, json.JSONDecodeError) as exc:
        raise JWTError("malformed header") from exc
    if not isinstance(header, dict) or header.get("alg") != "HS256":
        raise JWTError("rejected algorithm")

    signing_input = f"{header_b64}.{payload_b64}".encode("ascii")
    expected = hmac.new(secret, signing_input, hashlib.sha256).digest()
    try:
        actual = b64url_decode(sig_b64)
    except ValueError as exc:
        raise JWTError("malformed signature") from exc
    if len(actual) != len(expected) or not hmac.compare_digest(actual, expected):
        raise JWTError("bad signature")

    try:
        claims = json.loads(b64url_decode(payload_b64))
    except (ValueError, json.JSONDecodeError) as exc:
        raise JWTError("malformed payload") from exc
    if not isinstance(claims, dict):
        raise JWTError("malformed payload")

    if claims.get("typ") != ACCESS_TYP:
        raise JWTError("not an access token")

    clock = int(time.time() if now is None else now)
    exp = claims.get("exp")
    if not isinstance(exp, (int, float)):
        raise JWTError("missing exp")
    if clock > int(exp) + _LEEWAY_SECONDS:
        raise JWTError("expired")
    nbf = claims.get("nbf")
    if isinstance(nbf, (int, float)) and clock + _LEEWAY_SECONDS < int(nbf):
        raise JWTError("not yet valid")

    if issuer and claims.get("iss") != issuer:
        raise JWTError("wrong issuer")
    if audiences and not _audience_ok(claims.get("aud"), audiences):
        raise JWTError("wrong audience")

    sub = str(claims.get("sub") or "").strip()
    try:
        sub = str(uuid.UUID(sub))
    except ValueError as exc:
        raise JWTError("sub is not a UUID") from exc
    claims["sub"] = sub

    role = str(claims.get("global_role") or "").strip()
    if require_admin and role != ADMIN_ROLE:
        raise JWTError("not an admin")
    return claims


def _audience_ok(aud: Any, allowed: tuple[str, ...]) -> bool:
    if isinstance(aud, str):
        return aud in allowed
    if isinstance(aud, list):
        return any(isinstance(item, str) and item in allowed for item in aud)
    return False
