-- ============================================================================
-- oci-start Go rewrite: rollback for migration 0001
-- Drops all 42 tables in reverse declaration order.
-- ============================================================================

DROP TABLE IF EXISTS vps_monitor;
DROP TABLE IF EXISTS vpn_proxy_record;
DROP TABLE IF EXISTS traffic_alert;
DROP TABLE IF EXISTS tenant_social;
DROP TABLE IF EXISTS tenant_email_config;
DROP TABLE IF EXISTS tenant;
DROP TABLE IF EXISTS tem_instance;
DROP TABLE IF EXISTS telegram_user;
DROP TABLE IF EXISTS system_kv_store;
DROP TABLE IF EXISTS system_config;
DROP TABLE IF EXISTS server_metrics;
DROP TABLE IF EXISTS register_detail;
DROP TABLE IF EXISTS other_boot_instance;
DROP TABLE IF EXISTS open_boot_lock;
DROP TABLE IF EXISTS oci_multipart_upload_record;
DROP TABLE IF EXISTS oci_computer_info;
DROP TABLE IF EXISTS otp_key;
DROP TABLE IF EXISTS app_message;
DROP TABLE IF EXISTS memos;
DROP TABLE IF EXISTS login_user;
DROP TABLE IF EXISTS instance_traffic;
DROP TABLE IF EXISTS instance_detail;
DROP TABLE IF EXISTS instance_cloud_networks;
DROP TABLE IF EXISTS instance_backup_detail;
DROP TABLE IF EXISTS install_app;
DROP TABLE IF EXISTS email_send_record;
DROP TABLE IF EXISTS email_receive;
DROP TABLE IF EXISTS email_body;
DROP TABLE IF EXISTS dns_record;
DROP TABLE IF EXISTS db_configs;
DROP TABLE IF EXISTS console_connections;
DROP TABLE IF EXISTS cloud_tenancy;
DROP TABLE IF EXISTS cloud_ssh_folder;
DROP TABLE IF EXISTS oci_ssh_conn;
DROP TABLE IF EXISTS chat_ai_config;
DROP TABLE IF EXISTS boot_instance;
DROP TABLE IF EXISTS ban_record;
DROP TABLE IF EXISTS app_version;
DROP TABLE IF EXISTS ai_chat_history;
