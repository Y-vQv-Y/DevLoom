# VMware 真实部署测试环境

该目录用于从 Ubuntu 官方 2026-07-05 固定版本 cloud OVA 创建完全位于项目 `.vmware/` 下的测试虚拟机。脚本会校验该固定版本的官方 SHA-256，通过 VMware `guestinfo` 注入 cloud-init，安装 Docker、Compose、Git 和 open-vm-tools，并使用项目内 SSH 密钥执行后续构建。

```powershell
$vm = .\deploy\vmware\New-DevLoomVM.ps1
.\deploy\vmware\Invoke-DevLoomE2E.ps1 -VMIP $vm.IP
```

`.vmware/` 与 `.tools/` 已加入 Git 忽略，不会提交 OVA、虚拟磁盘、私钥或便携工具链。
