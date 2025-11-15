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
}

func runMigrations(db *gorm.DB) error {
	for i, stmt := range migrationStatements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}
	return nil
}
