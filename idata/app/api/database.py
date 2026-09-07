"""Small DB-API boundary that keeps a MySQL driver out of application code."""

from __future__ import annotations

from contextlib import contextmanager
from typing import Any, Callable, Iterator, Protocol

from .config import DatabaseSettings


class Connection(Protocol):
    def close(self) -> None: ...

    def commit(self) -> None: ...

    def rollback(self) -> None: ...


ConnectionFactory = Callable[[DatabaseSettings], Connection]


class Database:
    def __init__(
        self,
        settings: DatabaseSettings,
        connection_factory: ConnectionFactory,
    ) -> None:
        self._settings = settings
        self._connection_factory = connection_factory

    @contextmanager
    def connection(self) -> Iterator[Connection]:
        connection = self._connection_factory(self._settings)
        try:
            yield connection
            connection.commit()
        except Exception:
            connection.rollback()
            raise
        finally:
            connection.close()


def mysql_connection_options(settings: DatabaseSettings) -> dict[str, Any]:
    """Return driver-neutral options for the approved MySQL driver adapter."""
    options: dict[str, Any] = {
        "host": settings.host,
        "port": settings.port,
        "database": settings.database,
        "user": settings.user,
        "password": settings.password,
        "connect_timeout": settings.connect_timeout_seconds,
    }
    if settings.ssl_ca:
        options["ssl_ca"] = settings.ssl_ca
    return options
