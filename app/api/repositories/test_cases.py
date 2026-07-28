"""Queries for the test case table displayed by the web application."""

from __future__ import annotations

from typing import Any, Protocol


class DictionaryCursor(Protocol):
    def execute(self, operation: str, params: tuple[Any, ...] = ()) -> Any: ...

    def fetchall(self) -> list[dict[str, Any]]: ...


LIST_TEST_CASES_SQL = """
SELECT
    test_cases.public_id AS id,
    test_cases.title,
    test_cases.status,
    COALESCE(users.display_name, 'Unassigned') AS owner,
    test_cases.updated_at AS updated_at
FROM test_cases
LEFT JOIN users ON users.id = test_cases.owner_id
WHERE test_cases.deleted_at IS NULL
ORDER BY test_cases.updated_at DESC, test_cases.id DESC
LIMIT %s OFFSET %s
""".strip()


def list_test_cases(
    cursor: DictionaryCursor, *, limit: int = 100, offset: int = 0
) -> list[dict[str, Any]]:
    if not 1 <= limit <= 500:
        raise ValueError("limit must be between 1 and 500")
    if offset < 0:
        raise ValueError("offset must not be negative")

    cursor.execute(LIST_TEST_CASES_SQL, (limit, offset))
    return cursor.fetchall()
