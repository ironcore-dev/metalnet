<p>Packages:</p>
<ul>
<li>
<a href="#networking.metalnet.ironcore.dev%2fv1alpha1">networking.metalnet.ironcore.dev/v1alpha1</a>
</li>
</ul>
<h2 id="networking.metalnet.ironcore.dev/v1alpha1">networking.metalnet.ironcore.dev/v1alpha1</h2>
<div>
<p>Package v1alpha1 is the v1alpha1 version of the API.</p>
</div>
Resource Types:
<ul></ul>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.FirewallRule">FirewallRule
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceSpec">NetworkInterfaceSpec</a>)
</p>
<div>
<p>FirewallRule defines the desired state of FirewallRule</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>firewallRuleID</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/types#UID">
k8s.io/apimachinery/pkg/types.UID
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>direction</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.FirewallRuleDirection">
FirewallRuleDirection
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>action</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.FirewallRuleAction">
FirewallRuleAction
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>priority</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>ipFamily</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#ipfamily-v1-core">
Kubernetes core/v1.IPFamily
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>sourcePrefix</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">
IPPrefix
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>destinationPrefix</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">
IPPrefix
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>protocolMatch</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.ProtocolMatch">
ProtocolMatch
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.FirewallRuleAction">FirewallRuleAction
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.FirewallRule">FirewallRule</a>)
</p>
<div>
<p>FirewallRuleAction is the action of the rule.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Accept&#34;</p></td>
<td><p>FirewallRuleActionAccept is used to accept traffic.</p>
</td>
</tr><tr><td><p>&#34;Deny&#34;</p></td>
<td><p>FirewallRuleActionDeny is used to deny traffic.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.FirewallRuleDirection">FirewallRuleDirection
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.FirewallRule">FirewallRule</a>)
</p>
<div>
<p>FirewallRuleDirection is the direction of the rule.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Egress&#34;</p></td>
<td><p>FirewallRuleDirectionEgress is used to define rules for outgoing traffic.</p>
</td>
</tr><tr><td><p>&#34;Ingress&#34;</p></td>
<td><p>FirewallRuleDirectionIngress is used to define rules for incoming traffic.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.ICMPMatch">ICMPMatch
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.ProtocolMatch">ProtocolMatch</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>icmpType</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>icmpCode</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.IP">IP
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerSpec">LoadBalancerSpec</a>, <a href="#networking.metalnet.ironcore.dev/v1alpha1.NATDetails">NATDetails</a>, <a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceSpec">NetworkInterfaceSpec</a>, <a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceStatus">NetworkInterfaceStatus</a>)
</p>
<div>
<p>IP is an IP address.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>-</code><br/>
<em>
<a href="https://pkg.go.dev/net/netip#Addr">
net/netip.Addr
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">IPPrefix
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.FirewallRule">FirewallRule</a>, <a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceSpec">NetworkInterfaceSpec</a>, <a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceStatus">NetworkInterfaceStatus</a>, <a href="#networking.metalnet.ironcore.dev/v1alpha1.PeeredPrefix">PeeredPrefix</a>)
</p>
<div>
<p>IPPrefix represents a network prefix.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>-</code><br/>
<em>
<a href="https://pkg.go.dev/net/netip#Prefix">
net/netip.Prefix
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.LBPort">LBPort
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerSpec">LoadBalancerSpec</a>)
</p>
<div>
<p>LBPort consists of port and protocol</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>protocol</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.LoadBalancer">LoadBalancer
</h3>
<div>
<p>LoadBalancer is the Schema for the loadbalancers API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerSpec">
LoadBalancerSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>networkRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>NetworkRef is the Network this LoadBalancer is connected to</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerType">
LoadBalancerType
</a>
</em>
</td>
<td>
<p>Type defines whether the loadbalancer is using an internal or public ip</p>
</td>
</tr>
<tr>
<td>
<code>ipFamily</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#ipfamily-v1-core">
Kubernetes core/v1.IPFamily
</a>
</em>
</td>
<td>
<p>IPFamily defines which IPFamily this LoadBalancer is supporting</p>
</td>
</tr>
<tr>
<td>
<code>ip</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IP">
IP
</a>
</em>
</td>
<td>
<p>IP is the provided IP which should be loadbalanced by this LoadBalancer</p>
</td>
</tr>
<tr>
<td>
<code>ports</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.LBPort">
[]LBPort
</a>
</em>
</td>
<td>
<p>Ports are the provided ports</p>
</td>
</tr>
<tr>
<td>
<code>nodeName</code><br/>
<em>
string
</em>
</td>
<td>
<p>NodeName is the name of the node on which the LoadBalancer should be created.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerStatus">
LoadBalancerStatus
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerSpec">LoadBalancerSpec
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancer">LoadBalancer</a>)
</p>
<div>
<p>LoadBalancerSpec defines the desired state of LoadBalancer</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>networkRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>NetworkRef is the Network this LoadBalancer is connected to</p>
</td>
</tr>
<tr>
<td>
<code>type</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerType">
LoadBalancerType
</a>
</em>
</td>
<td>
<p>Type defines whether the loadbalancer is using an internal or public ip</p>
</td>
</tr>
<tr>
<td>
<code>ipFamily</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#ipfamily-v1-core">
Kubernetes core/v1.IPFamily
</a>
</em>
</td>
<td>
<p>IPFamily defines which IPFamily this LoadBalancer is supporting</p>
</td>
</tr>
<tr>
<td>
<code>ip</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IP">
IP
</a>
</em>
</td>
<td>
<p>IP is the provided IP which should be loadbalanced by this LoadBalancer</p>
</td>
</tr>
<tr>
<td>
<code>ports</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.LBPort">
[]LBPort
</a>
</em>
</td>
<td>
<p>Ports are the provided ports</p>
</td>
</tr>
<tr>
<td>
<code>nodeName</code><br/>
<em>
string
</em>
</td>
<td>
<p>NodeName is the name of the node on which the LoadBalancer should be created.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerState">LoadBalancerState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerStatus">LoadBalancerStatus</a>)
</p>
<div>
<p>LoadBalancerState is the binding state of a LoadBalancer.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Error&#34;</p></td>
<td><p>LoadBalancerStateError is used for any LoadBalancer that is some error occurred.</p>
</td>
</tr><tr><td><p>&#34;Pending&#34;</p></td>
<td><p>LoadBalancerStatePending is used for any LoadBalancer that is in an intermediate state.</p>
</td>
</tr><tr><td><p>&#34;Ready&#34;</p></td>
<td><p>LoadBalancerStateReady is used for any LoadBalancer that is ready.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerStatus">LoadBalancerStatus
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancer">LoadBalancer</a>)
</p>
<div>
<p>LoadBalancerStatus defines the observed state of LoadBalancer</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerState">
LoadBalancerState
</a>
</em>
</td>
<td>
<p>State is the LoadBalancerState of the LoadBalancer.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerType">LoadBalancerType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.LoadBalancerSpec">LoadBalancerSpec</a>)
</p>
<div>
<p>LoadBalancerType is the type of a LoadBalancer.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Internal&#34;</p></td>
<td><p>LoadBalancerTypeInternal is used for any LoadBalancer that uses private IPs.</p>
</td>
</tr><tr><td><p>&#34;Public&#34;</p></td>
<td><p>LoadBalancerTypePublic is used for any LoadBalancer that uses public IPs.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.LocalUIDReference">LocalUIDReference
</h3>
<div>
<p>LocalUIDReference is a reference to another entity including its UID</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>name</code><br/>
<em>
string
</em>
</td>
<td>
<p>Name is the name of the referenced entity.</p>
</td>
</tr>
<tr>
<td>
<code>uid</code><br/>
<em>
<a href="https://pkg.go.dev/k8s.io/apimachinery/pkg/types#UID">
k8s.io/apimachinery/pkg/types.UID
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>UID is the UID of the referenced entity.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.NATDetails">NATDetails
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceSpec">NetworkInterfaceSpec</a>, <a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceStatus">NetworkInterfaceStatus</a>)
</p>
<div>
<p>LBPort consists of port and protocol</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>ip</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IP">
IP
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>port</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>endPort</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.Network">Network
</h3>
<div>
<p>Network is the Schema for the networks API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkSpec">
NetworkSpec
</a>
</em>
</td>
<td>
<br/>
<br/>
<table>
<tr>
<td>
<code>id</code><br/>
<em>
int32
</em>
</td>
<td>
<p>ID is the unique identifier of the Network</p>
</td>
</tr>
<tr>
<td>
<code>peeredIDs</code><br/>
<em>
[]int32
</em>
</td>
<td>
<p>PeeredIDs are the IDs of networks to peer with.</p>
</td>
</tr>
<tr>
<td>
<code>peeredPrefixes</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.PeeredPrefix">
[]PeeredPrefix
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PeeredPrefixes are the allowed CIDRs of the peered networks.</p>
</td>
</tr>
</table>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.NetworkInterface">NetworkInterface
</h3>
<div>
<p>NetworkInterface is the Schema for the networkinterfaces API</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>metadata</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#objectmeta-v1-meta">
Kubernetes meta/v1.ObjectMeta
</a>
</em>
</td>
<td>
Refer to the Kubernetes API documentation for the fields of the
<code>metadata</code> field.
</td>
</tr>
<tr>
<td>
<code>spec</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceSpec">
NetworkInterfaceSpec
</a>
</em>
</td>
<td>
<p>Spec defines the desired state of NetworkInterface.</p>
<br/>
<br/>
<table>
<tr>
<td>
<code>networkRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>NetworkRef is the Network this NetworkInterface is connected to</p>
</td>
</tr>
<tr>
<td>
<code>ipFamilies</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#ipfamily-v1-core">
[]Kubernetes core/v1.IPFamily
</a>
</em>
</td>
<td>
<p>IPFamilies defines which IPFamilies this NetworkInterface is supporting
Only one IP supported at the moment.</p>
</td>
</tr>
<tr>
<td>
<code>ips</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IP">
[]IP
</a>
</em>
</td>
<td>
<p>IPs are the provided IPs or EphemeralIPs which should be assigned to this NetworkInterface
Only one IP supported at the moment.</p>
</td>
</tr>
<tr>
<td>
<code>virtualIP</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IP">
IP
</a>
</em>
</td>
<td>
<p>Virtual IP</p>
</td>
</tr>
<tr>
<td>
<code>prefixes</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">
[]IPPrefix
</a>
</em>
</td>
<td>
<p>Prefixes are the provided Prefix</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerTargets</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">
[]IPPrefix
</a>
</em>
</td>
<td>
<p>Loadbalancer Targets are the provided Prefix</p>
</td>
</tr>
<tr>
<td>
<code>nat</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.NATDetails">
NATDetails
</a>
</em>
</td>
<td>
<p>NATInfo is detailed information about the NAT on this interface</p>
</td>
</tr>
<tr>
<td>
<code>nodeName</code><br/>
<em>
string
</em>
</td>
<td>
<p>NodeName is the name of the node on which the interface should be created.</p>
</td>
</tr>
<tr>
<td>
<code>firewallRules</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.FirewallRule">
[]FirewallRule
</a>
</em>
</td>
<td>
<p>FirewallRules are the firewall rules to be applied to this interface.</p>
</td>
</tr>
</table>
</td>
</tr>
<tr>
<td>
<code>status</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceStatus">
NetworkInterfaceStatus
</a>
</em>
</td>
<td>
<p>Status defines the observed state of NetworkInterface.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceSpec">NetworkInterfaceSpec
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterface">NetworkInterface</a>)
</p>
<div>
<p>NetworkInterfaceSpec defines the desired state of NetworkInterface</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>networkRef</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#localobjectreference-v1-core">
Kubernetes core/v1.LocalObjectReference
</a>
</em>
</td>
<td>
<p>NetworkRef is the Network this NetworkInterface is connected to</p>
</td>
</tr>
<tr>
<td>
<code>ipFamilies</code><br/>
<em>
<a href="https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.27/#ipfamily-v1-core">
[]Kubernetes core/v1.IPFamily
</a>
</em>
</td>
<td>
<p>IPFamilies defines which IPFamilies this NetworkInterface is supporting
Only one IP supported at the moment.</p>
</td>
</tr>
<tr>
<td>
<code>ips</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IP">
[]IP
</a>
</em>
</td>
<td>
<p>IPs are the provided IPs or EphemeralIPs which should be assigned to this NetworkInterface
Only one IP supported at the moment.</p>
</td>
</tr>
<tr>
<td>
<code>virtualIP</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IP">
IP
</a>
</em>
</td>
<td>
<p>Virtual IP</p>
</td>
</tr>
<tr>
<td>
<code>prefixes</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">
[]IPPrefix
</a>
</em>
</td>
<td>
<p>Prefixes are the provided Prefix</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerTargets</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">
[]IPPrefix
</a>
</em>
</td>
<td>
<p>Loadbalancer Targets are the provided Prefix</p>
</td>
</tr>
<tr>
<td>
<code>nat</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.NATDetails">
NATDetails
</a>
</em>
</td>
<td>
<p>NATInfo is detailed information about the NAT on this interface</p>
</td>
</tr>
<tr>
<td>
<code>nodeName</code><br/>
<em>
string
</em>
</td>
<td>
<p>NodeName is the name of the node on which the interface should be created.</p>
</td>
</tr>
<tr>
<td>
<code>firewallRules</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.FirewallRule">
[]FirewallRule
</a>
</em>
</td>
<td>
<p>FirewallRules are the firewall rules to be applied to this interface.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceState">NetworkInterfaceState
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceStatus">NetworkInterfaceStatus</a>)
</p>
<div>
<p>NetworkInterfaceState is the binding state of a NetworkInterface.</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;Error&#34;</p></td>
<td><p>NetworkInterfaceStateError is used for any NetworkInterface that is some error occurred.</p>
</td>
</tr><tr><td><p>&#34;Pending&#34;</p></td>
<td><p>NetworkInterfaceStatePending is used for any NetworkInterface that is in an intermediate state.</p>
</td>
</tr><tr><td><p>&#34;Ready&#34;</p></td>
<td><p>NetworkInterfaceStateReady is used for any NetworkInterface that is ready.</p>
</td>
</tr></tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceStatus">NetworkInterfaceStatus
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterface">NetworkInterface</a>)
</p>
<div>
<p>NetworkInterfaceStatus defines the observed state of NetworkInterface</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>pciAddress</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.PCIAddress">
PCIAddress
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>virtualIP</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IP">
IP
</a>
</em>
</td>
<td>
<p>VirtualIP is any virtual ip assigned to the NetworkInterface.</p>
</td>
</tr>
<tr>
<td>
<code>natIP</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.NATDetails">
NATDetails
</a>
</em>
</td>
<td>
<p>NatIP is detailed information about the NAT on this interface</p>
</td>
</tr>
<tr>
<td>
<code>prefixes</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">
[]IPPrefix
</a>
</em>
</td>
<td>
<p>Prefixes are the Prefixes reserved for this NetworkInterface</p>
</td>
</tr>
<tr>
<td>
<code>loadBalancerTargets</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">
[]IPPrefix
</a>
</em>
</td>
<td>
<p>LoadBalancerTargets are the Targets reserved for this NetworkInterface</p>
</td>
</tr>
<tr>
<td>
<code>state</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceState">
NetworkInterfaceState
</a>
</em>
</td>
<td>
<p>State is the NetworkInterfaceState of the NetworkInterface.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.NetworkSpec">NetworkSpec
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.Network">Network</a>)
</p>
<div>
<p>NetworkSpec defines the desired state of Network</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code><br/>
<em>
int32
</em>
</td>
<td>
<p>ID is the unique identifier of the Network</p>
</td>
</tr>
<tr>
<td>
<code>peeredIDs</code><br/>
<em>
[]int32
</em>
</td>
<td>
<p>PeeredIDs are the IDs of networks to peer with.</p>
</td>
</tr>
<tr>
<td>
<code>peeredPrefixes</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.PeeredPrefix">
[]PeeredPrefix
</a>
</em>
</td>
<td>
<em>(Optional)</em>
<p>PeeredPrefixes are the allowed CIDRs of the peered networks.</p>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.PCIAddress">PCIAddress
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkInterfaceStatus">NetworkInterfaceStatus</a>)
</p>
<div>
<p>PCIAddress is a PCI address.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>domain</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>bus</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>slot</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>function</code><br/>
<em>
string
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.PeeredPrefix">PeeredPrefix
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.NetworkSpec">NetworkSpec</a>)
</p>
<div>
<p>PeeredPrefix contains information of the peered networks and their allowed CIDRs.</p>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>id</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>prefixes</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.IPPrefix">
[]IPPrefix
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.PortMatch">PortMatch
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.ProtocolMatch">ProtocolMatch</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>srcPort</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>endSrcPort</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>dstPort</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>endDstPort</code><br/>
<em>
int32
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.ProtocolMatch">ProtocolMatch
</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.FirewallRule">FirewallRule</a>)
</p>
<div>
</div>
<table>
<thead>
<tr>
<th>Field</th>
<th>Description</th>
</tr>
</thead>
<tbody>
<tr>
<td>
<code>protocolType</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.ProtocolType">
ProtocolType
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>icmp</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.ICMPMatch">
ICMPMatch
</a>
</em>
</td>
<td>
</td>
</tr>
<tr>
<td>
<code>portRange</code><br/>
<em>
<a href="#networking.metalnet.ironcore.dev/v1alpha1.PortMatch">
PortMatch
</a>
</em>
</td>
<td>
</td>
</tr>
</tbody>
</table>
<h3 id="networking.metalnet.ironcore.dev/v1alpha1.ProtocolType">ProtocolType
(<code>string</code> alias)</h3>
<p>
(<em>Appears on:</em><a href="#networking.metalnet.ironcore.dev/v1alpha1.ProtocolMatch">ProtocolMatch</a>)
</p>
<div>
<p>ProtocolType is the type for the network protocol</p>
</div>
<table>
<thead>
<tr>
<th>Value</th>
<th>Description</th>
</tr>
</thead>
<tbody><tr><td><p>&#34;ICMP&#34;</p></td>
<td><p>FirewallRuleProtocolTypeICMP is used for ICMP traffic.</p>
</td>
</tr><tr><td><p>&#34;TCP&#34;</p></td>
<td><p>FirewallRuleProtocolTypeTCP is used for TCP traffic.</p>
</td>
</tr><tr><td><p>&#34;UDP&#34;</p></td>
<td><p>FirewallRuleProtocolTypeUDP is used for UDP traffic.</p>
</td>
</tr></tbody>
</table>
<hr/>
<p><em>
Generated with <code>gen-crd-api-reference-docs</code>
</em></p>
