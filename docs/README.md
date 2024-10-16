# Metalnet Documentation

## Developement and automated testing
* [Environment setup](./development/setup.md)
* [Test](./development/test.md)
* [Contribution guide](./development/contribution.md)

## API references
* [`networking.ironcore.dev` API Group](./api-reference/networking.md)

## CRD usage
* [Usage](./usage/crd_usage.md)

## Multiport-eswitch mode
When running dpservice with Mellanox in multiport-eswitch mode, it is important to tell metalnet about it:
```
metalnet --multiport-eswitch
```
or (overrides the above)
```
echo -n "eswitch" > /var/lib/metalnet/mode
```
This changes the way metalnet generates identifiers for virtual function representors that are sent over to dpservice.

If pf1-proxy is also in use, it is important to mark it as used in the metalnet VF database:
```
mkdir -p /var/lib/metalnet/netfns/claims
echo -n "$pf1_proxy_vf_pci" > /var/lib/metalnet/netfns/claims/00000001-0000-4000-0000-000000000000
```
Where the `$pf1_proxy_vf_pci` is the PCI address of the VF representor for pf1-proxy. This should be the only VF using `mlx5_core` driver instead of the `vfio-pci` driver. One of many ways to retrieve such address is as follows:
```
pf1_proxy_vf_name=$(/opt/local/bin/dpdk-devbind.py -s | grep "mlx5Gen Virtual Function" | grep "drv=mlx5_core" | awk -F'if=' '{print $2}' | awk '{print $1}')
pf1_proxy_vf_pci=$(/opt/local/bin/dpdk-devbind.py -s | grep "mlx5Gen Virtual Function" | grep $pf1_proxy_vf_name | awk '{print $1}')
```
