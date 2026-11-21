# Legacy cfg CI artifact verification — 2026-11-18

GitHub package run 44311837662 succeeded for exact source head f665e20fc718af5f628cbc9b3b75ccf988ee7293. Artifact 14430722338 (`debs-vyatta-cfg`) is 3,588,948 bytes; downloaded ZIP SHA256 `2d08849d1e830a24778c857f5241d7a5402c0c983077c0625da4f87d22979cae` matches GitHub metadata. ZIP contained flat names. `verify-ci-artifact.py` checked all 23 `.changes` payload sizes and SHA256 hashes; Source is `vyatta-cfg`, Version 1.36. The `.dsc` archive hash/size matched; all 20 binary `.deb` metadata versions are 1.36.

Unpacked `vyatta-save-config.pl` is mode 0755, SHA256 `a9c16a880919336bda48fe664fcb3191c35decc7b0fa6d4ebb420481476c9232`, identical to the reviewed `f665e20` Git blob. No packages were installed. ZIP and payloads remain excluded local runtime artifacts; this evidence is not an appliance acceptance or release.

Verified `.changes` payload hashes:

```text
735faefb5ce1febbec94d08386e711666163d19f4d91c753a7189cf1aab0988f vyatta-cfg_1.36.dsc
82c2988f7ff4fa85e70d0bedd4bb74e75651946b78792cf99d56a983ac6eb3b8 vyatta-cfg_1.36.tar.xz
d882ba612deaf3ddd6749c04b6edf6787a76d3f6db4994515d611858a8621903 cli-shell-api_1.36_amd64.deb
affbc63d488a6dc78236468bf3a2970ca3a2f3961f8d79738cb2632796033a96 libvyatta-cfg-dbg_1.36_amd64.deb
907bbd23a7414afc7e81e1b7939cf999b2731eeefa18e5a34795f7dd02b2b890 libvyatta-cfg-dev_1.36_amd64.deb
8c7a2a881d1af1810f93c82304c9cf69addeeb3495333594568d9c2b0c848bb7 libvyatta-cfgclient-perl_1.36_amd64.deb
773a360c690d101e41b6f6a7456acdea06b1910cbf65d748fdc5652adf02afb4 libvyatta-config-perl_1.36_amd64.deb
9bac2ac5f6f34e055c3af98f3ec4c4e48c86084e44e6db35d6ce96d3a9239728 libvyatta-config2_1.36_amd64.deb
461a9b09c0ee646bfbfcc089763bf699a470c92325f36437f37ea8e7de156f91 python3-vyatta-cfgclient_1.36_amd64.deb
a4db35c80551faaddcce59da01a4f4f909633f8a89a2da2febe7b3c11d5aa58f vcli_1.36_all.deb
a72bec38abc5a992c28ca502313ff67e8812dcc6c3bd87a099c17eba54dd5ef0 vyatta-cfg_1.36_amd64.buildinfo
12a2caedb36756c7ea3af5f0ce221b732e9fdca8b362412ba052270fcf5560e9 vyatta-cfg_1.36_amd64.deb
21e606198e2fec7d3b194eb59ac0c6a8569c322ddb84b87f2b42592be8a371a0 vyatta-config-shell_1.36_all.deb
c8dd2e8a1924efc5d1da0b8134ca826c9c4d46870ca9aa60c7f07a88844c74e2 vyatta-dscp-v1-yang_1.36_all.deb
3455e255e33ae1aeb901a2cad06c4e5b8dbffcc80e3a6639ad5efdc032261283 vyatta-op-config-v1-yang_1.36_all.deb
86add84ccd3c76892789868d8f7b60322473bd55deac9cc7cd8900f621cc2b80 vyatta-op-show-confirmed-commit-v1-yang_1.36_all.deb
3a24074f41fb6632b0211b3d02648eee5fceb2d383c45e018daee5f98b88030b vyatta-policy-v1-yang_1.36_all.deb
350afc228f4c8b1bff9c872383692c8ebf2b45febf02d55cd5ba55cafe10310a vyatta-resources-v1-yang_1.36_all.deb
2c8b1568260cfaf5a8aed5f79acfdda005d7fefb8695eee6de38348db361a4b4 vyatta-security-v1-yang_1.36_all.deb
3a0700306ab162185b4fa44c907f45ce4267b399e6248b7b43f6fc112544762c vyatta-services-v1-yang_1.36_all.deb
3cea3b5b741b51a09759bcb0ccd338d60250bf904bf54bb4e84fcb5b7d50bd88 vyatta-system-acm-v1-yang_1.36_all.deb
6cc7b89079ddd2c2836a30de034235fca03409d9cca89ed333f3836009dac705 vyatta-system-v1-yang_1.36_all.deb
cc9e2f91ca97d06dd23f5cdd2bf0a77b24963416c12624236158a15529f2eeb2 vyatta-types-v1-yang_1.36_all.deb
source: vyatta-cfg version: 1.36
```
