package rustack_terraform

import (
	"context"
	"log"

	"fmt"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/pilat/rustack-go/rustack"
)

func resourceRustackPort() *schema.Resource {
	args := Defaults()
	args.injectContextVdcById()
	args.injectCreatePort()

	return &schema.Resource{
		CreateContext: resourceRustackPortCreate,
		ReadContext:   resourceRustackPortRead,
		UpdateContext: resourceRustackPortUpdate,
		DeleteContext: resourceRustackPortDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},
		Schema: args,
	}
}

func resourceRustackPortCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	manager := meta.(*CombinedConfig).rustackManager()
	targetVdc, err := GetVdcById(d, manager)
	if err != nil {
		return diag.Errorf("vdc_id: Error getting VDC: %s", err)
	}
	portNetwork, err := GetNetworkById(d, manager, nil)

	firewallsCount := d.Get("firewall_templates.#").(int)
	firewalls := make([]*rustack.FirewallTemplate, firewallsCount)
	firewallsResourceData := d.Get("firewall_templates").(*schema.Set).List()
	for j, firewallId := range firewallsResourceData {
		portFirewall, err := manager.GetFirewallTemplate(firewallId.(string))
		if err != nil {
			return diag.Errorf("firewall_templates: Error getting Firewall Template: %s", err)
		}
		firewalls[j] = portFirewall
	}

	ipAddressStr := d.Get("ip_address").(string)
	if ipAddressStr == "" {
		ipAddressStr = "0.0.0.0"
	}

	log.Printf("[DEBUG] subnetInfo: %#v", targetVdc)
	newPort := rustack.NewPort(portNetwork, firewalls, ipAddressStr)
	fmt.Println(ipAddressStr)
	targetVdc.WaitLock()
	if err = targetVdc.CreateEmptyPort(&newPort); err != nil {
		return diag.Errorf("Error creating port: %s", err)
	}
	newPort.WaitLock()
	d.SetId(newPort.ID)
	fmt.Println(ipAddressStr)
	log.Printf("[INFO] Port created, ID: %s", d.Id())

	return resourceRustackPortRead(ctx, d, meta)
}

func resourceRustackPortRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	manager := meta.(*CombinedConfig).rustackManager()
	port, err := manager.GetPort(d.Id())
	if err != nil {
		return diag.Errorf("id: Error getting port: %s", err)
	}

	d.SetId(port.ID)
	d.Set("ip_address", port.IpAddress)
	d.Set("network_id", port.Network)

	firewalls := make([]*string, len(port.FirewallTemplates))
	for i, firewall := range port.FirewallTemplates {
		firewalls[i] = &firewall.ID
	}

	d.Set("firewall_templates", firewalls)

	return nil
}

func resourceRustackPortUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	manager := meta.(*CombinedConfig).rustackManager()

	portId := d.Id()
	port, err := manager.GetPort(portId)
	if err != nil {
		return diag.Errorf("id: Error getting port: %s", err)
	}
	ip_address := d.Get("ip_address").(string)
	if ip_address != *port.IpAddress {
		err = port.UpdateIpAddress(&ip_address)
		if err != nil {
			return diag.Errorf("ip_address: Error updating ip_address: %s", err)
		}
	}

	if d.HasChange("firewall_templates") {
		firewallsCount := d.Get("firewall_templates.#").(int)
		firewalls := make([]*rustack.FirewallTemplate, firewallsCount)
		firewallsResourceData := d.Get("firewall_templates").(*schema.Set).List()
		for j, firewallId := range firewallsResourceData {
			portFirewall, err := manager.GetFirewallTemplate(firewallId.(string))
			if err != nil {
				return diag.Errorf("firewall_templates: Error updating Firewall Template: %s", err)
			}
			firewalls[j] = portFirewall
		}

		if err = port.UpdateFirewall(firewalls); err != nil {
			return diag.FromErr(err)
		}
	}
	port.WaitLock()
	return resourceRustackPortRead(ctx, d, meta)
}

func resourceRustackPortDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	manager := meta.(*CombinedConfig).rustackManager()
	portId := d.Id()

	port, err := manager.GetPort(portId)
	if err != nil {
		return diag.Errorf("id: Error getting port: %s", err)
	}

	err = port.ForceDelete()
	if err != nil {
		return diag.Errorf("Error deleting port: %s", err)
	}
	port.WaitLock()

	d.SetId("")
	log.Printf("[INFO] Port deleted, ID: %s", portId)
	return nil
}
