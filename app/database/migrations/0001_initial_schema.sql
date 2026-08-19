-- Initial IDATA schema for MySQL 8.0 or later.

CREATE TABLE users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    public_id CHAR(36) NOT NULL,
    display_name VARCHAR(120) NOT NULL,
    email VARCHAR(254) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_users_public_id (public_id),
    UNIQUE KEY uq_users_email (email)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_cases (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    public_id VARCHAR(32) NOT NULL,
    title VARCHAR(500) NOT NULL,
    status ENUM('Passed', 'Failed', 'Blocked', 'Not run') NOT NULL DEFAULT 'Not run',
    owner_id BIGINT UNSIGNED NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at TIMESTAMP(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_test_cases_public_id (public_id),
    KEY ix_test_cases_status (status),
    KEY ix_test_cases_owner_id (owner_id),
    CONSTRAINT fk_test_cases_owner
        FOREIGN KEY (owner_id) REFERENCES users (id)
        ON UPDATE RESTRICT ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_suites (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    public_id VARCHAR(32) NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT NOT NULL,
    owner_id BIGINT UNSIGNED NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),
    deleted_at TIMESTAMP(6) NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uq_test_suites_public_id (public_id),
    KEY ix_test_suites_owner_id (owner_id),
    CONSTRAINT fk_test_suites_owner
        FOREIGN KEY (owner_id) REFERENCES users (id)
        ON UPDATE RESTRICT ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_suite_cases (
    test_suite_id BIGINT UNSIGNED NOT NULL,
    test_case_id BIGINT UNSIGNED NOT NULL,
    position INT UNSIGNED NOT NULL DEFAULT 0,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (test_suite_id, test_case_id),
    UNIQUE KEY uq_test_suite_cases_position (test_suite_id, position),
    KEY ix_test_suite_cases_test_case_id (test_case_id),
    CONSTRAINT fk_test_suite_cases_suite
        FOREIGN KEY (test_suite_id) REFERENCES test_suites (id)
        ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT fk_test_suite_cases_case
        FOREIGN KEY (test_case_id) REFERENCES test_cases (id)
        ON UPDATE RESTRICT ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_runs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    public_id VARCHAR(32) NOT NULL,
    name VARCHAR(200) NOT NULL,
    device VARCHAR(255) NOT NULL,
    notes TEXT NULL,
    status ENUM('Ready', 'Running', 'Completed', 'Cancelled')
        NOT NULL DEFAULT 'Ready',
    created_by BIGINT UNSIGNED NULL,
    started_at TIMESTAMP(6) NULL,
    completed_at TIMESTAMP(6) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_test_runs_public_id (public_id),
    KEY ix_test_runs_status_created_at (status, created_at),
    KEY ix_test_runs_created_by (created_by),
    CONSTRAINT fk_test_runs_created_by
        FOREIGN KEY (created_by) REFERENCES users (id)
        ON UPDATE RESTRICT ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE test_run_cases (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    test_run_id BIGINT UNSIGNED NOT NULL,
    test_case_id BIGINT UNSIGNED NOT NULL,
    source_suite_id BIGINT UNSIGNED NULL,
    position INT UNSIGNED NOT NULL DEFAULT 0,
    status ENUM('Passed', 'Failed', 'Blocked', 'Not run') NOT NULL DEFAULT 'Not run',
    notes TEXT NULL,
    executed_by BIGINT UNSIGNED NULL,
    executed_at TIMESTAMP(6) NULL,
    created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    updated_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
        ON UPDATE CURRENT_TIMESTAMP(6),
    PRIMARY KEY (id),
    UNIQUE KEY uq_test_run_cases_case (test_run_id, test_case_id),
    UNIQUE KEY uq_test_run_cases_position (test_run_id, position),
    KEY ix_test_run_cases_test_case_id (test_case_id),
    KEY ix_test_run_cases_source_suite_id (source_suite_id),
    KEY ix_test_run_cases_executed_by (executed_by),
    CONSTRAINT fk_test_run_cases_run
        FOREIGN KEY (test_run_id) REFERENCES test_runs (id)
        ON UPDATE RESTRICT ON DELETE CASCADE,
    CONSTRAINT fk_test_run_cases_case
        FOREIGN KEY (test_case_id) REFERENCES test_cases (id)
        ON UPDATE RESTRICT ON DELETE RESTRICT,
    CONSTRAINT fk_test_run_cases_source_suite
        FOREIGN KEY (source_suite_id) REFERENCES test_suites (id)
        ON UPDATE RESTRICT ON DELETE SET NULL,
    CONSTRAINT fk_test_run_cases_executed_by
        FOREIGN KEY (executed_by) REFERENCES users (id)
        ON UPDATE RESTRICT ON DELETE SET NULL
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

CREATE TABLE schema_migrations (
    version VARCHAR(255) NOT NULL,
    applied_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_0900_ai_ci;

INSERT INTO schema_migrations (version) VALUES ('0001_initial_schema');
