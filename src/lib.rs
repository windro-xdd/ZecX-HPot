// Library facade so integration tests and other crates can use the modules.
// Linux-only: fail compilation on other platforms unless dev feature enabled.
#[cfg(all(not(target_os = "linux"), not(feature = "allow-nonlinux-dev")))]
compile_error!("proto-honeypot currently supports only Linux targets (use feature 'allow-nonlinux-dev' for development)");
pub mod config;
pub mod fsgen;
pub mod listener;
pub mod reporter;
pub mod util;
pub mod metrics;

// Re-export commonly used types
pub use config::Config;
pub use listener::ConnLog;
pub use reporter::Reporter;
