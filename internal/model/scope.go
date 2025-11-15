package model

import "github.com/google/uuid"

type ScopeType string

const (
	ScopeCity       ScopeType = "CITY"
	ScopeKgu        ScopeType = "KGU"
	ScopeContractor ScopeType = "CONTRACTOR"
	ScopeDriver     ScopeType = "DRIVER"
	ScopeTechnical  ScopeType = "TECHNICAL"
)

type Scope struct {
	Type            ScopeType
	OrgID           *uuid.UUID
	ContractorIDs   []uuid.UUID
	OrganizationIDs []uuid.UUID
	DriverID        *uuid.UUID
	TechnicalOnly   bool
}

func (s Scope) AllowsViolation(contractorID *uuid.UUID) bool {
	if s.Type == ScopeCity || s.Type == ScopeTechnical {
		return true
	}
	if contractorID == nil {
		return false
	}
	if s.Type == ScopeContractor && s.OrgID != nil {
		return *s.OrgID == *contractorID
	}
	for _, id := range s.ContractorIDs {
		if id == *contractorID {
			return true
		}
	}
	return false
}
