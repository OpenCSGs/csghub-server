package types

import "testing"

func TestClassifyRepositoryLicensePatterns(t *testing.T) {
	tests := []struct {
		name       string
		license    string
		status     ComplianceStatus
		permission CommercialPermission
	}{
		{name: "MIT", license: "mit", status: ComplianceStatusCompliant, permission: CommercialPermissionAllowed},
		{name: "Apache 2.0", license: "apache-2.0", status: ComplianceStatusCompliant, permission: CommercialPermissionAllowed},
		{name: "GPL or later", license: "gpl-3.0-or-later", status: ComplianceStatusCompliant, permission: CommercialPermissionCopyleft},
		{name: "GPL only", license: " GPL-2.0-ONLY ", status: ComplianceStatusCompliant, permission: CommercialPermissionCopyleft},
		{name: "LGPL version", license: "lgpl-2.0", status: ComplianceStatusCompliant, permission: CommercialPermissionCopyleft},
		{name: "CC non-commercial no derivatives", license: "cc-by-nc-nd-2.0", status: ComplianceStatusCompliant, permission: CommercialPermissionNonCommercial},
		{name: "CC non-commercial share alike", license: "cc-by-nc-sa-1.0", status: ComplianceStatusCompliant, permission: CommercialPermissionNonCommercial},
		{name: "unrelated GPL text", license: "xgpl-3.0", status: ComplianceStatusPendingReview, permission: CommercialPermissionCustomTerms},
		{name: "incomplete CC family", license: "cc-by-nc-nd", status: ComplianceStatusPendingReview, permission: CommercialPermissionCustomTerms},
		{name: "different CC family", license: "cc-by-nc-ndx-4.0", status: ComplianceStatusPendingReview, permission: CommercialPermissionCustomTerms},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, permission := ClassifyRepositoryLicense(test.license)
			if status != test.status || permission != test.permission {
				t.Fatalf("ClassifyRepositoryLicense(%q) = (%q, %q), want (%q, %q)", test.license, status, permission, test.status, test.permission)
			}
		})
	}
}
