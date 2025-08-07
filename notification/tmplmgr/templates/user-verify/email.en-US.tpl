{{/* title section */}}
Personal Certification
---
{{/* content section */}}
<html>
    <body>
        <h3>Personal Certification</h3>
        {{if eq .verify_status "approved"}}
            <p>Your Personal Certification application has been approved. Thank you for your support!</p>
        {{else if eq .verify_status "rejected"}}
            <p>Your Personal Certification application has been rejected. Please contact the system administrator!</p>
        {{else}}
            <p>Your Personal Certification application status has been updated.</p>
        {{end}}
    </body>
</html> 