# Corrected login CI artifact verification — 2026-11-18

GitHub Actions run 44314453839 passed for exact reviewed source head `9a622e2afbc6b44254b849dccb6aab111a3fe92b`. Artifact 14432905024 (`debs-vyatta-login`) is 154830 bytes; the downloaded ZIP SHA256 `2d94a51d4a52daafaec06e11656d677a3a20983ed4c4f1720c8cb59d83bf2c5a` matches GitHub artifact metadata. The ZIP contained only flat filenames. Existing `verify-ci-artifact.py` checked the SHA256 and size of all 13 files listed in its sole `.changes`; Source is `vyatta-login` and Version is `2.46`. The `.dsc` source archive SHA256 and size agree with that `.changes`. All ten binary packages report version 2.46 via `dpkg-deb -f`.

The unpacked main package's RPC hash is `00df845f5f52e2b336a686d70e9744b278192bf5939112da6e05022e9e58c7bd`, mode 0755; the helper hash is `316940c19f571c2553f5f4baae7463bf54bea637c7b900a585a768e812335d64`, mode 0644. Both match the normalized reviewed production source hashes. Main package Depends includes configd >= 2.17, Breaks includes vyatta-system-login-v1-yang << 2.45, and that YANG package Depends on vyatta-login >= 2.45. Packages were inspected in a disposable guest, not installed. This artifact is not a release, and actual appliance acceptance remains pending.

`.changes` SHA256 payloads:

```text
f86986f04acb2e910d8e2fc1e72647e0296847103b455a6c91239c13384b911c vyatta-login_2.46.dsc
c0aac591e7d0b6569f6e0757ff7df836ce17cf4f579b8ef6580311adac8c4c23 vyatta-login_2.46.tar.xz
91f0dde959c101c525dd1142360c578a96a1caffb2fa219ff85f5ec28c3d42dd vyatta-login_2.46_amd64.buildinfo
4cf0547b5031a6f81cacc480affda68313fd184bfdacbed2682bd0a84e9e95c3 vyatta-login_2.46_amd64.deb
7f3178953d6cfaa02915f67f00ed83b63dcc1abe9ba25876031675e934edcb3d vyatta-op-reset-system-login-v1-yang_2.46_all.deb
13c3eb97c66bddd991bf15525c1782f4d566960e24c0f40918ff753279f76d5a vyatta-op-show-system-login-history-v1-yang_2.46_all.deb
0ba841ee3693f26e0644a11527ed540c87f59330665ac92e5545077d116b9f71 vyatta-op-show-system-login-v1-yang_2.46_all.deb
9ef9e9b1b0d14838abc8fc9def9a88a0756c90bde9d9a68faea813bdc5683e03 vyatta-password-renewal-dbgsym_2.46_amd64.deb
79a860ea977733c35777d241503dfc186991ec2c8ee109525279c31652ced8f2 vyatta-password-renewal_2.46_amd64.deb
26600373dce3abb53db8530448e07a263a1db0d9b0c40e493f4d2a2df3d9d92a vyatta-radius-system-user-login-feature_2.46_all.deb
9fe6d1f203ddcd151a16cf7364771c213ea2f08dc8343634bc2aeae3deda5b74 vyatta-system-login-history-v1-yang_2.46_all.deb
752ba66cc49651371ce6ca30a49c5f97e7ef34b2e0083bf1df5fc4114adaa4ca vyatta-system-login-radius-routing-instance-v1-yang_2.46_all.deb
f9dc2c7aa2a42a181f92d8786ea4a503d4342cf577ec58d0d5b979c5c3f11664 vyatta-system-login-v1-yang_2.46_all.deb
```

The ZIP and extracted packages remain local runtime artifacts and are excluded from this checkpoint.
