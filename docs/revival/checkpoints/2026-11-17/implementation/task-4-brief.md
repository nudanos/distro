## Task 4: Network migration and applied state

**Owner/Files:** vyatta-kernel-forwarding, ifmgrd, routing-instance, vrf-manager, vyatta-protocols-frr.
**Consumes:** current boot hook/ifmgrd transitions, VRF naming and MPLS state.
**Produces:** fail-closed migration of unsupported security policy; truthful applied state/error reporting; consistent VRF limits; MPLS cleanup.
- [ ] Reproduce imported deny-policy removal using existing fixture/parser tests and isolated packet-level VM test.
- [ ] Add a boot-wide transit guard/recovery contract before removing unsupported security policy; do not merely return a hook error that loader ignores.
- [ ] Make ifmgrd distinguish changed/success/failure, retain previous applied state and notify/retry accurately after action failure.
- [ ] Align VRF schema/runtime limits and return failure on provisioning refusal.
- [ ] Reconcile previous/new MPLS inputs, disable removed interfaces, propagate sysctl errors.
- [ ] Run boundary/deletion/failure tests plus existing scenarios; commit and write task-4-report.md.

