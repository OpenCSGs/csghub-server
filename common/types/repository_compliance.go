package types

import (
	"path"
	"strings"
)

// ComplianceStatus describes the review state of a model or dataset license.
type ComplianceStatus string

const (
	ComplianceStatusCompliant     ComplianceStatus = "compliant"
	ComplianceStatusPendingReview ComplianceStatus = "pending_review"
	ComplianceStatusNonCompliant  ComplianceStatus = "non_compliant"
)

// IsValid reports whether status is a supported compliance status.
func (status ComplianceStatus) IsValid() bool {
	switch status {
	case ComplianceStatusCompliant, ComplianceStatusPendingReview, ComplianceStatusNonCompliant:
		return true
	default:
		return false
	}
}

// CommercialPermission describes how a model or dataset may be used commercially.
type CommercialPermission string

const (
	CommercialPermissionAllowed       CommercialPermission = "commercial_allowed"
	CommercialPermissionConditional   CommercialPermission = "conditional_commercial"
	CommercialPermissionNonCommercial CommercialPermission = "non_commercial"
	CommercialPermissionCopyleft      CommercialPermission = "copyleft"
	CommercialPermissionCustomTerms   CommercialPermission = "custom_terms"
)

// IsValid reports whether permission is a supported commercial permission.
func (permission CommercialPermission) IsValid() bool {
	switch permission {
	case CommercialPermissionAllowed,
		CommercialPermissionConditional,
		CommercialPermissionNonCommercial,
		CommercialPermissionCopyleft,
		CommercialPermissionCustomTerms:
		return true
	default:
		return false
	}
}

// SupportsLicenseCompliance reports whether the repository type participates in
// license compliance classification.
func SupportsLicenseCompliance(repoType RepositoryType) bool {
	return repoType == ModelRepo || repoType == DatasetRepo
}

// NormalizeRepositoryLicense returns the canonical form used for matching and
// detecting semantic license changes.
func NormalizeRepositoryLicense(license string) string {
	return strings.ToLower(strings.TrimSpace(license))
}

// ClassifyRepositoryLicense maps a declared license identifier to the platform's
// compliance and commercial-use classifications. Unknown, missing, and custom
// licenses always require manual review.
func ClassifyRepositoryLicense(license string) (ComplianceStatus, CommercialPermission) {
	normalizedLicense := NormalizeRepositoryLicense(license)
	switch normalizedLicense {
	case "apache-2.0", "mit", "afl-3.0", "ecl-2.0", "cc0-1.0", "cc-by-4.0":
		return ComplianceStatusCompliant, CommercialPermissionAllowed
	case "creativeml-openrail-m":
		return ComplianceStatusCompliant, CommercialPermissionConditional
	case "cc-by-nc-4.0":
		return ComplianceStatusCompliant, CommercialPermissionNonCommercial
	case "gpl", "agpl-3.0", "lgpl":
		return ComplianceStatusCompliant, CommercialPermissionCopyleft
	}

	if strings.HasPrefix(normalizedLicense, "cc-by-nc-nd-") ||
		strings.HasPrefix(normalizedLicense, "cc-by-nc-sa-") {
		return ComplianceStatusCompliant, CommercialPermissionNonCommercial
	}
	if strings.HasPrefix(normalizedLicense, "gpl-") || strings.HasPrefix(normalizedLicense, "lgpl-") {
		return ComplianceStatusCompliant, CommercialPermissionCopyleft
	}
	return ComplianceStatusPendingReview, CommercialPermissionCustomTerms
}

// ClassifyRepositoryLicenseForType limits automatic license classification to
// repository types covered by the policy.
func ClassifyRepositoryLicenseForType(repoType RepositoryType, license string) (ComplianceStatus, CommercialPermission) {
	if !SupportsLicenseCompliance(repoType) {
		return ComplianceStatusPendingReview, CommercialPermissionCustomTerms
	}
	return ClassifyRepositoryLicense(license)
}

// IsRootLicenseDocument reports whether a path names a conventional license
// document at the repository root.
func IsRootLicenseDocument(fileName string) bool {
	cleaned := strings.TrimPrefix(path.Clean(strings.ReplaceAll(strings.TrimSpace(fileName), "\\", "/")), "./")
	if cleaned == "" || cleaned == "." || strings.Contains(cleaned, "/") {
		return false
	}
	upperName := strings.ToUpper(cleaned)
	for _, prefix := range []string{"LICENSE", "LICENCE"} {
		if upperName == prefix || strings.HasPrefix(upperName, prefix+".") ||
			strings.HasPrefix(upperName, prefix+"-") || strings.HasPrefix(upperName, prefix+"_") {
			return true
		}
	}
	return false
}

// IsRootReadme reports whether a path names README.md at the repository root.
func IsRootReadme(fileName string) bool {
	cleaned := strings.TrimPrefix(path.Clean(strings.ReplaceAll(strings.TrimSpace(fileName), "\\", "/")), "./")
	return strings.EqualFold(cleaned, ReadmeFileName)
}
