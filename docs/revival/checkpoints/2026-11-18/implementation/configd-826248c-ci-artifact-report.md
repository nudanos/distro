# Configd CI artifact verification — 2026-11-18

GitHub package run 44316942009 succeeded for exact source head `826248c62b3d14840ab20150c3aa42c502038d97`. Artifact 14432718658 (`debs-configd`) is 49,315,057 bytes; downloaded ZIP SHA256 `e45d41c7a74933aa967112374bcf662edb0e4e1ac792a6ac1de54296ab3f135e` matches GitHub metadata. ZIP entries were flat. Existing `verify-ci-artifact.py` checked all 17 `.changes` payload sizes and hashes; Source is `configd`, Version 2.18. The `.dsc` source archive checksum and size agree; all 14 binary `.deb` metadata versions are 2.18.

Source archive members `pkg/server/config_mgmt.go` and `pkg/startuplock/startup_lock.go` have SHA256 `382a391fa1f3ee8bbd87b69b871338c887dd1cd5663a9851c51c5074a7ecd435` and `ae1850b3ccafa42f590b3cc584192c81cb3d4fd3c9b8f0e644d73e577503b183`, matching exact 826248c Git blobs. The generated package postinst retains the guarded `try-restart configd.service` after debhelper sections. Parent verified merged commit a58fbcb3afe23aa5867dcca2d1b34e2c4064a51e has the same tree as 826248c. Packages were not installed. ZIP and payloads remain excluded local runtime artifacts; this is not appliance acceptance or a release.

Verified `.changes` payload hashes:

```text
076a19170d0bbe9d2c3f9bdd41a3740893b55062b06340a9059faf0a6d4dcc53 configd_2.18.dsc
b90d6883d1889af02c20b6bcdf67eeb1283595c7591e7be7f5a11ce94ae93956 configd_2.18.tar.gz
49bc0c2100dc8bed7c1611df89995d7d6a79537302397c739e102541e4a9d0e4 config-utils_2.18_amd64.deb
7f8146a6c0b9c970e91296d0eb19bd09aa8c5aa087eb6318f081e3cfafcb6c7f configd-v1-yang_2.18_all.deb
2e7a405cbb359b032608264c3ee71e6bf6ac27cab0b6a2bdbd3636a7660bae0f configd_2.18_amd64.buildinfo
21b889a52df4cf7b3b9cc8f20ccc6ae1c85522ddd3029a99176c71f44f7b9e39 configd_2.18_amd64.deb
a41c852c8cdacb7f32499f85a6574d6445245d8f20ff70740db73123cb289602 golang-github-danos-configd-client-dev_2.18_all.deb
118cfe17654529f823849cd6cf346f216b4a96474540859c1b34e582e40d73a2 golang-github-danos-configd-rpc-dev_2.18_all.deb
64c75700e019545e1b0abf610cb3af7aefa7e0bc7ad27d846ab14b43aa63ff60 ietf-inet-types-yang_2.18_all.deb
b526f74b70ec085bcd01d33e5a77fd33c3af6e4c7600ebedf964ca03a892f5e2 ietf-netconf-monitoring-yang_2.18_all.deb
34cb6542648cf9886179168971d05b764ab86ba3fcaa79309431e80a4f98ecc9 ietf-netconf-with-defaults-yang_2.18_all.deb
0ca0372e67091266a9e3b511158b4462a293514fcc48d9a918ab082cb0e57187 ietf-netconf-yang_2.18_all.deb
91b4203c7c1a16df728c7576190adfc18fc94b7e7bdc2e08ee8da1cba84d04b6 ietf-yang-types-yang_2.18_all.deb
22aa4f613444c2cfcb74b7b65ea35b62b999f2ac20178152feb11db725916bbb vyatta-system-acm-configd-v1-yang_2.18_all.deb
a1ce815012f7f4163cdb3558344ae9adaa318c1f6886207a7d72459fc6994d0a yang-utils_2.18_amd64.deb
93cbd86e9aec5b7877e7b2e1fcd1f5df7ad6e0101c2f3baa9123804261e180cc yangd-v1-yang_2.18_all.deb
a5a61ef3dfae5467e7a0669a2042224c73c50f49165bf57c64c87abe98fcae1d yangd_2.18_amd64.deb
source: configd version: 2.18
```
