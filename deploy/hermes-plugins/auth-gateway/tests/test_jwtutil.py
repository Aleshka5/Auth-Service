from __future__ import annotations

import base64
import hashlib
import hmac
import json
import sys
import time
import unittest
import uuid
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from jwtutil import JWTError, verify_access_jwt  # noqa: E402

SECRET = b"test-secret-key-32-bytes-long!!"
ISSUER = "https://filenkov.store"
AUD = ("filenkov.store", "localhost")


def _b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b"=").decode("ascii")


def mint(
    *,
    sub: str | None = None,
    role: str = "ADMIN",
    typ: str = "access",
    iss: str = ISSUER,
    aud: object = "filenkov.store",
    exp: int | None = None,
    nbf: int | None = None,
    now: int | None = None,
    secret: bytes = SECRET,
    alg: str = "HS256",
    extra: dict | None = None,
) -> str:
    clock = int(time.time() if now is None else now)
    header = {"alg": alg, "typ": "JWT"}
    payload = {
        "sub": sub or str(uuid.uuid4()),
        "typ": typ,
        "iss": iss,
        "aud": aud,
        "email": "boss@b.c",
        "username": "boss",
        "global_role": role,
        "exp": clock + 900 if exp is None else exp,
        "nbf": clock if nbf is None else nbf,
        "iat": clock,
    }
    if extra:
        payload.update(extra)
    h = _b64url(json.dumps(header, separators=(",", ":")).encode())
    p = _b64url(json.dumps(payload, separators=(",", ":")).encode())
    if alg == "none":
        return f"{h}.{p}."
    sig = hmac.new(secret, f"{h}.{p}".encode("ascii"), hashlib.sha256).digest()
    return f"{h}.{p}.{_b64url(sig)}"


class VerifyAccessJWTTests(unittest.TestCase):
    def test_valid_admin_token(self):
        sub = str(uuid.uuid4())
        claims = verify_access_jwt(
            mint(sub=sub),
            secret=SECRET,
            issuer=ISSUER,
            audiences=AUD,
        )
        self.assertEqual(claims["sub"], sub)
        self.assertEqual(claims["global_role"], "ADMIN")

    def test_rejects_none_alg(self):
        with self.assertRaises(JWTError):
            verify_access_jwt(
                mint(alg="none"),
                secret=SECRET,
                issuer=ISSUER,
                audiences=AUD,
            )

    def test_rejects_bad_signature(self):
        with self.assertRaises(JWTError):
            verify_access_jwt(
                mint(secret=b"other-secret-key-32-bytes-long!!"),
                secret=SECRET,
                issuer=ISSUER,
                audiences=AUD,
            )

    def test_rejects_expired(self):
        now = 1_700_000_000
        with self.assertRaises(JWTError):
            verify_access_jwt(
                mint(exp=now - 60, now=now),
                secret=SECRET,
                issuer=ISSUER,
                audiences=AUD,
                now=now,
            )

    def test_rejects_stranger(self):
        with self.assertRaises(JWTError):
            verify_access_jwt(
                mint(role="STRANGER"),
                secret=SECRET,
                issuer=ISSUER,
                audiences=AUD,
            )

    def test_rejects_non_uuid_sub(self):
        with self.assertRaises(JWTError):
            verify_access_jwt(
                mint(sub="not-a-uuid"),
                secret=SECRET,
                issuer=ISSUER,
                audiences=AUD,
            )

    def test_rejects_refresh_typ(self):
        with self.assertRaises(JWTError):
            verify_access_jwt(
                mint(typ="refresh"),
                secret=SECRET,
                issuer=ISSUER,
                audiences=AUD,
            )

    def test_rejects_wrong_issuer(self):
        with self.assertRaises(JWTError):
            verify_access_jwt(
                mint(iss="https://evil.example"),
                secret=SECRET,
                issuer=ISSUER,
                audiences=AUD,
            )

    def test_accepts_aud_list(self):
        claims = verify_access_jwt(
            mint(aud=["filenkov.store", "localhost"]),
            secret=SECRET,
            issuer=ISSUER,
            audiences=AUD,
        )
        self.assertEqual(claims["global_role"], "ADMIN")


if __name__ == "__main__":
    unittest.main()
