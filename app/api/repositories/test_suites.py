"""Queries for the test suite table displayed by the web application."""

from __future__ import annotations

from typing import Any

from .test_cases import DictionaryCursor


LIST_TEST_SUITES_SQL = """
SELECT
    test_suites.public_id AS id,
    test_suites.name,
    test_suites.description,
    COALESCE(users.display_name, 'Unassigned') AS owner,
    test_suites.updated_at AS updated_at,
    GROUP_CONCAT(
        test_cases.public_id
        ORDER BY test_suite_cases.position
        SEPARATOR ','
    ) AS case_ids
FROM test_suites
LEFT JOIN users ON users.id = test_suites.owner_id
LEFT JOIN test_suite_cases ON test_suite_cases.test_suite_id = test_suites.id
LEFT JOIN test_cases
    ON test_cases.id = test_suite_cases.test_case_id
    AND test_cases.deleted_at IS NULL
WHERE test_suites.deleted_at IS NULL
GROUP BY
    test_suites.id,
    test_suites.public_id,
    test_suites.name,
    test_suites.description,
    users.display_name,
    test_suites.updated_at
ORDER BY test_suites.updated_at DESC, test_suites.id DESC
LIMIT %s OFFSET %s
""".strip()


def list_test_suites(
    cursor: DictionaryCursor, *, limit: int = 100, offset: int = 0
) -> list[dict[str, Any]]:
    if not 1 <= limit <= 500:
        raise ValueError("limit must be between 1 and 500")
    if offset < 0:
        raise ValueError("offset must not be negative")

    cursor.execute(LIST_TEST_SUITES_SQL, (limit, offset))
    rows = cursor.fetchall()
    for row in rows:
        case_ids = row.pop("case_ids", None)
        row["caseIds"] = case_ids.split(",") if case_ids else []
    return rows
