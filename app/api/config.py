"""Environment-based configuration for the future remote MySQL connection."""

from __future__ import annotations

from dataclasses import dataclass
import os


@dataclass(frozen=True)
class DatabaseSettings:
    host: str
    port: int
    database: str
    user: str
    password: str
    connect_timeout_seconds: int = 10
    ssl_ca: str | None = None

    @classmethod
    def from_environment(cls) -> "DatabaseSettings":
        required_names = (
            "IDATA_DB_HOST",
            "IDATA_DB_NAME",
            "IDATA_DB_USER",
            "IDATA_DB_PASSWORD",
        )
        missing = [name for name in required_names if not os.environ.get(name)]
        if missing:
            names = ", ".join(missing)
            raise RuntimeError(f"Missing required database environment variables: {names}")

        return cls(
            host=os.environ["IDATA_DB_HOST"],
            port=_positive_integer("IDATA_DB_PORT", default=3306),
            database=os.environ["IDATA_DB_NAME"],
            user=os.environ["IDATA_DB_USER"],
            password=os.environ["IDATA_DB_PASSWORD"],
            connect_timeout_seconds=_positive_integer(
                "IDATA_DB_CONNECT_TIMEOUT", default=10
            ),
            ssl_ca=os.environ.get("IDATA_DB_SSL_CA") or None,
        )


def _positive_integer(name: str, *, default: int) -> int:
    raw_value = os.environ.get(name, str(default))
    try:
        value = int(raw_value)
    except ValueError as error:
        raise RuntimeError(f"{name} must be an integer") from error

    if value <= 0:
        raise RuntimeError(f"{name} must be greater than zero")
    return value
