package db

import (
	"fmt"

	"gorm.io/gorm"
)

var migrationStatements = []string{
	`CREATE EXTENSION IF NOT EXISTS "uuid-ossp";`,
	`CREATE EXTENSION IF NOT EXISTS "pgcrypto";`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'violation_status') THEN
			CREATE TYPE violation_status AS ENUM ('OPEN', 'CANCELED', 'FIXED');
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'violation_severity') THEN
			CREATE TYPE violation_severity AS ENUM ('LOW', 'MEDIUM', 'HIGH');
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'violation_detected_by') THEN
			CREATE TYPE violation_detected_by AS ENUM ('LPR', 'VOLUME', 'GPS', 'SYSTEM');
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'appeal_status') THEN
			CREATE TYPE appeal_status AS ENUM ('SUBMITTED', 'UNDER_REVIEW', 'NEED_INFO', 'APPROVED', 'REJECTED', 'CLOSED');
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'appeal_reason_code') THEN
			CREATE TYPE appeal_reason_code AS ENUM ('CAMERA_ERROR', 'TRANSIT_PATH', 'WRONG_ASSIGNMENT', 'OTHER');
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'attachment_file_type') THEN
			CREATE TYPE attachment_file_type AS ENUM ('IMAGE', 'VIDEO', 'DOC');
		END IF;
	END
	$$;`,
	`CREATE TABLE IF NOT EXISTS violations (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
		type VARCHAR(64) NOT NULL,
		detected_by violation_detected_by NOT NULL,
		severity violation_severity NOT NULL,
		status violation_status NOT NULL DEFAULT 'OPEN',
		description TEXT,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_violations_trip_id ON violations (trip_id);`,
	`CREATE INDEX IF NOT EXISTS idx_violations_status ON violations (status);`,
	`CREATE INDEX IF NOT EXISTS idx_violations_detected_by ON violations (detected_by);`,
	`CREATE INDEX IF NOT EXISTS idx_violations_created_at ON violations (created_at);`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM information_schema.columns WHERE table_name = 'trips' AND column_name = 'violation_reason') THEN
			ALTER TABLE trips ADD COLUMN violation_reason TEXT;
		END IF;
	END
	$$;`,
	`CREATE TABLE IF NOT EXISTS violation_appeals (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		violation_id UUID NOT NULL REFERENCES violations(id) ON DELETE CASCADE,
		trip_id UUID NOT NULL REFERENCES trips(id) ON DELETE CASCADE,
		ticket_id UUID REFERENCES tickets(id) ON DELETE SET NULL,
		driver_id UUID REFERENCES drivers(id) ON DELETE SET NULL,
		contractor_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
		reason_code appeal_reason_code NOT NULL,
		reason_text TEXT NOT NULL,
		status appeal_status NOT NULL DEFAULT 'SUBMITTED',
		resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
		resolved_at TIMESTAMPTZ,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_violation_appeals_violation_id ON violation_appeals (violation_id);`,
	`CREATE INDEX IF NOT EXISTS idx_violation_appeals_status ON violation_appeals (status);`,
	`CREATE INDEX IF NOT EXISTS idx_violation_appeals_reason_code ON violation_appeals (reason_code);`,
	`CREATE UNIQUE INDEX IF NOT EXISTS uniq_violation_active_appeal
		ON violation_appeals (violation_id)
		WHERE status IN ('SUBMITTED', 'UNDER_REVIEW', 'NEED_INFO');`,
	`CREATE TABLE IF NOT EXISTS violation_appeal_attachments (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		appeal_id UUID NOT NULL REFERENCES violation_appeals(id) ON DELETE CASCADE,
		file_url TEXT NOT NULL,
		file_type attachment_file_type NOT NULL,
		uploaded_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_violation_attachments_appeal_id ON violation_appeal_attachments (appeal_id);`,
	`CREATE TABLE IF NOT EXISTS violation_appeal_comments (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		appeal_id UUID NOT NULL REFERENCES violation_appeals(id) ON DELETE CASCADE,
		author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
		author_role VARCHAR(32) NOT NULL,
		message TEXT NOT NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_violation_comments_appeal_id ON violation_appeal_comments (appeal_id);`,
	`CREATE TABLE IF NOT EXISTS violation_status_log (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		violation_id UUID NOT NULL REFERENCES violations(id) ON DELETE CASCADE,
		old_status violation_status,
		new_status violation_status NOT NULL,
		note TEXT,
		changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_violation_status_log_violation_id ON violation_status_log (violation_id);`,
	`CREATE TABLE IF NOT EXISTS appeal_status_log (
		id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
		appeal_id UUID NOT NULL REFERENCES violation_appeals(id) ON DELETE CASCADE,
		old_status appeal_status,
		new_status appeal_status NOT NULL,
		note TEXT,
		changed_by UUID REFERENCES users(id) ON DELETE SET NULL,
		created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
	);`,
	`CREATE INDEX IF NOT EXISTS idx_appeal_status_log_appeal_id ON appeal_status_log (appeal_id);`,
	`CREATE OR REPLACE FUNCTION set_row_updated_at()
	RETURNS TRIGGER AS $$
	BEGIN
		NEW.updated_at = NOW();
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_violations_updated_at') THEN
			CREATE TRIGGER trg_violations_updated_at
				BEFORE UPDATE ON violations
				FOR EACH ROW
				EXECUTE PROCEDURE set_row_updated_at();
		END IF;
		IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_violation_appeals_updated_at') THEN
			CREATE TRIGGER trg_violation_appeals_updated_at
				BEFORE UPDATE ON violation_appeals
				FOR EACH ROW
				EXECUTE PROCEDURE set_row_updated_at();
		END IF;
	END
	$$;`,
	`CREATE OR REPLACE FUNCTION map_trip_status_to_violation(status TEXT)
	RETURNS TABLE(v_type VARCHAR, v_detected violation_detected_by, v_severity violation_severity) AS $$
	BEGIN
		RETURN QUERY SELECT
			CAST(CASE status
				WHEN 'ROUTE_VIOLATION' THEN 'ROUTE_VIOLATION'
				WHEN 'FOREIGN_AREA' THEN 'FOREIGN_AREA'
				WHEN 'MISMATCH_PLATE' THEN 'MISMATCH_PLATE'
				WHEN 'OVER_CAPACITY' THEN 'OVER_CAPACITY'
				WHEN 'NO_AREA_WORK' THEN 'NO_AREA_WORK'
				WHEN 'NO_ASSIGNMENT' THEN 'NO_AREA_WORK'
				WHEN 'SUSPICIOUS_VOLUME' THEN 'OVER_CAPACITY'
				WHEN 'OVER_CONTRACT_LIMIT' THEN 'OVER_CONTRACT_LIMIT'
				ELSE 'SYSTEM' END AS VARCHAR) AS v_type,
			CAST(CASE status
				WHEN 'MISMATCH_PLATE' THEN 'LPR'
				WHEN 'ROUTE_VIOLATION' THEN 'GPS'
				WHEN 'FOREIGN_AREA' THEN 'GPS'
				WHEN 'SUSPICIOUS_VOLUME' THEN 'VOLUME'
				WHEN 'OVER_CAPACITY' THEN 'VOLUME'
				ELSE 'SYSTEM' END AS violation_detected_by) AS v_detected,
			CAST(CASE status
				WHEN 'ROUTE_VIOLATION' THEN 'HIGH'
				WHEN 'FOREIGN_AREA' THEN 'HIGH'
				WHEN 'OVER_CAPACITY' THEN 'HIGH'
				WHEN 'SUSPICIOUS_VOLUME' THEN 'MEDIUM'
				WHEN 'MISMATCH_PLATE' THEN 'MEDIUM'
				WHEN 'NO_ASSIGNMENT' THEN 'MEDIUM'
				WHEN 'NO_AREA_WORK' THEN 'MEDIUM'
				ELSE 'LOW' END AS violation_severity) AS v_severity;
	END;
	$$ LANGUAGE plpgsql;`,
	`CREATE OR REPLACE FUNCTION trg_trips_set_violation_reason()
	RETURNS TRIGGER AS $$
	BEGIN
		IF NEW.status <> 'OK'
			AND (OLD.status IS NULL OR OLD.status = 'OK')
			AND (NEW.violation_reason IS NULL OR NEW.violation_reason = '') THEN
			NEW.violation_reason := CONCAT('Auto violation: ', NEW.status);
		END IF;
		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_trips_set_violation_reason') THEN
			CREATE TRIGGER trg_trips_set_violation_reason
				BEFORE UPDATE OF status ON trips
				FOR EACH ROW
				WHEN (NEW.status <> 'OK' AND (OLD.status IS NULL OR OLD.status = 'OK'))
				EXECUTE PROCEDURE trg_trips_set_violation_reason();
		END IF;
	END
	$$;`,
	`CREATE OR REPLACE FUNCTION trg_trips_auto_violation()
	RETURNS TRIGGER AS $$
	DECLARE
		v_type VARCHAR;
		v_detected violation_detected_by;
		v_severity violation_severity;
		v_description TEXT;
		v_existing UUID;
		v_new_id UUID;
	BEGIN
		IF NEW.status = 'OK' THEN
			RETURN NEW;
		END IF;
		IF TG_OP = 'UPDATE' AND (OLD.status = NEW.status) THEN
			RETURN NEW;
		END IF;
		SELECT result.v_type, result.v_detected, result.v_severity
			INTO v_type, v_detected, v_severity
		FROM map_trip_status_to_violation(NEW.status) AS result;

		IF v_type IS NULL THEN
			RETURN NEW;
		END IF;

		v_description := COALESCE(NEW.violation_reason, CONCAT('Auto violation: ', NEW.status));

		SELECT id INTO v_existing
		FROM violations
		WHERE trip_id = NEW.id AND type = v_type AND status = 'OPEN'
		LIMIT 1;

		IF v_existing IS NOT NULL THEN
			RETURN NEW;
		END IF;

		INSERT INTO violations (trip_id, type, detected_by, severity, status, description, created_at, updated_at)
		VALUES (NEW.id, v_type, v_detected, v_severity, 'OPEN', v_description, NOW(), NOW())
		RETURNING id INTO v_new_id;

		INSERT INTO violation_status_log (violation_id, old_status, new_status, note, changed_by, created_at)
		VALUES (v_new_id, NULL, 'OPEN', 'auto from trip status', NULL, NOW());

		RETURN NEW;
	END;
	$$ LANGUAGE plpgsql;`,
	`DO $$
	BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname = 'trg_trips_auto_violation') THEN
			CREATE TRIGGER trg_trips_auto_violation
				AFTER UPDATE OF status ON trips
				FOR EACH ROW
				WHEN (NEW.status <> 'OK')
				EXECUTE PROCEDURE trg_trips_auto_violation();
		END IF;
	END
	$$;`,
}

func runMigrations(db *gorm.DB) error {
	for i, stmt := range migrationStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}
