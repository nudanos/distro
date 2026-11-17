## Task 5: Authenticated image update

**Owner/Files:** vyatta-image-tools and distro release scripts/workflows.
**Consumes:** shipped NuDanOS public key and existing image-fetch/install interface.
**Produces:** signed image/release manifest and required verification before mounting/install; explicit recorded local-development mode only if needed.
- [ ] Reproduce absent/wrong-key/corrupt signature behavior with current helper tests.
- [ ] Require dedicated-key verification, remove silent signature downgrade, define explicit development override and compatibility migration.
- [ ] Publish matching signed assets without exposing private keys to untrusted build code.
- [ ] Test valid/missing/corrupt/wrong-key/expired inputs and release/install compatibility.
- [ ] Run tests, commit and write task-5-report.md.

