# OCI RuntimeSpec DRA Driver

An 🧪 experimental DRA driver that allows setting arbitrary oci runtime spec fields
into pods via configuration associated with a `ResourceClaim`.

It abuses the CDI container edits API to inject an oci hook into the container
creation lifecycle which acts based on a user-provided config.
