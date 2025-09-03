use crate::config::Config;
use anyhow::Result;

pub fn print_iptables_suggestions(cfg: &Config) {
    println!("Iptables suggestion (run as root to apply):");
    println!("# Redirect incoming traffic to this host only for listed ports");
    for p in &cfg.ports {
        println!("iptables -A INPUT -p tcp --dport {} -j ACCEPT", p);
    }
    println!("# Drop other inbound connection by default (use with care)");
    println!("iptables -A INPUT -p tcp -j DROP");
}

pub fn apply_iptables_rules(cfg: &Config) -> Result<()> {
    // Applying rules requires root and platform support — we'll shell out to iptables.
    // For safety in this scaffold we simply print what we would run and return Ok.
    println!("apply_iptables_rules: (not executing in scaffold) would apply rules for {} ports", cfg.ports.len());
    Ok(())
}
