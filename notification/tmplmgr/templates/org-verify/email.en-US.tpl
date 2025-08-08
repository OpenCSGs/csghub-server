{{/* title section */}}
Organization Certification
---
{{/* content section */}}
<html>
    <body>
        <h3>Organization Certification</h3>
        {{if eq .verify_status "approved"}}
            <p>Your Organization Certification application has been approved. Thank you for your support!</p>
        {{else if eq .verify_status "rejected"}}
            <p>Your Organization Certification application has been rejected. Please contact the system administrator!</p>
        {{else}}
            <p>Your Organization Certification application status has been updated.</p>
        {{end}}
    </body>
</html> 