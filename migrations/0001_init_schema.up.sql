-- ============================================================================
-- oci-start Go rewrite: initial schema (migration 0001)
-- Reverse-engineered from 42 JPA entities in
--   oci-dao/src/main/java/com/doubledimple/dao/entity/
-- SQLite is type-affine; declared types express intent.
-- No foreign keys (entities use plain scalar cross-refs).
-- ============================================================================

-- entity: AiChatHistory.java
CREATE TABLE ai_chat_history (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    model_id TEXT,
    created_at TEXT NOT NULL
);
CREATE INDEX idx_user_id ON ai_chat_history(user_id);
CREATE INDEX idx_user_created ON ai_chat_history(user_id, created_at);

-- entity: AppVersion.java
CREATE TABLE app_version (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    current_version TEXT NOT NULL,
    latest_version TEXT NOT NULL,
    deploy_type TEXT NOT NULL,
    create_time TEXT NOT NULL,
    update_time TEXT NOT NULL
);

-- entity: BanRecord.java
CREATE TABLE ban_record (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ip_address TEXT NOT NULL,
    source TEXT,
    operator_name TEXT,
    reason TEXT,
    status INTEGER NOT NULL,
    create_time TEXT NOT NULL,
    unban_time TEXT,
    remark TEXT
);
CREATE INDEX idx_ip ON ban_record(ip_address);
CREATE INDEX idx_status ON ban_record(status);
CREATE INDEX idx_ip_status ON ban_record(ip_address, status);

-- entity: BootInstance.java
CREATE TABLE boot_instance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version BIGINT NOT NULL DEFAULT 0,
    boot_id TEXT,
    tenant_id BIGINT,
    ocpu INTEGER,
    memory INTEGER,
    disk INTEGER,
    loop_time INTEGER,
    instance_count INTEGER,
    status INTEGER,
    architecture TEXT,
    root_password TEXT,
    public_ip TEXT,
    next_execution_time TEXT,
    add_count BIGINT DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    remark TEXT,
    created_at TEXT,
    updated_at TEXT,
    cloud_type INTEGER DEFAULT 1,
    current_attempt_count INTEGER DEFAULT 0,
    yesterday_attempt_count INTEGER DEFAULT 0,
    reset_today_flag INTEGER DEFAULT 0,
    last_reset_date TEXT,
    fail_count INTEGER DEFAULT 0,
    total_count BIGINT,
    image_id TEXT,
    operating_system TEXT,
    operating_system_version TEXT,
    data_gap TEXT,
    notify_flag TEXT
);

-- entity: ChatAiConfig.java
CREATE TABLE chat_ai_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version BIGINT NOT NULL DEFAULT 0,
    tenant_id TEXT,
    model_id TEXT,
    show_model_id TEXT,
    cloud_type INTEGER,
    model_name TEXT,
    provider TEXT,
    api_key TEXT,
    base_url TEXT,
    enabled INTEGER,
    system_prompt TEXT,
    max_tokens INTEGER,
    temperature TEXT,
    max_history_messages INTEGER DEFAULT 10,
    created_at TEXT,
    updated_at TEXT
);

-- entity: CloudSshConn.java
CREATE TABLE oci_ssh_conn (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT,
    name TEXT NOT NULL,
    remark TEXT NOT NULL,
    username TEXT NOT NULL,
    host TEXT,
    port INTEGER,
    password TEXT,
    cloud_type INTEGER DEFAULT 1,
    folder_id BIGINT
);

-- entity: CloudSshFolder.java
CREATE TABLE cloud_ssh_folder (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    parent_id BIGINT,
    sort_order INTEGER DEFAULT 0,
    created_at TEXT,
    updated_at TEXT,
    deleted INTEGER NOT NULL
);

-- entity: CloudTenancy.java
CREATE TABLE cloud_tenancy (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenancy_name TEXT NOT NULL,
    cloud_type INTEGER NOT NULL,
    type INTEGER NOT NULL DEFAULT 1,
    def_name TEXT,
    account_cost TEXT,
    create_time TEXT,
    update_time TEXT
);

-- entity: ConsoleConnection.java
CREATE TABLE console_connections (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT NOT NULL,
    tenant_id BIGINT NOT NULL,
    connection_id TEXT NOT NULL,
    private_key_path TEXT,
    cloud_type INTEGER DEFAULT 1
);

-- entity: DbConfig.java
CREATE TABLE db_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT NOT NULL UNIQUE,
    db_name TEXT,
    db_private_url TEXT,
    db_public_url TEXT,
    db_port INTEGER,
    db_password TEXT,
    db_id TEXT,
    db_version TEXT,
    db_storage_size INTEGER,
    db_data_base_mode TEXT,
    db_display_name TEXT,
    db_shape_name TEXT,
    db_availability_domain TEXT,
    db_high_available INTEGER,
    db_subnet_id TEXT,
    cloud_type INTEGER DEFAULT 1,
    db_type INTEGER DEFAULT 1,
    db_status TEXT,
    create_at TEXT,
    updated_at TEXT
);

-- entity: DnsRecord.java
CREATE TABLE dns_record (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    provider_type TEXT NOT NULL,
    domain_name TEXT NOT NULL,
    record_name TEXT NOT NULL,
    record_type TEXT NOT NULL,
    record_value TEXT NOT NULL,
    ttl INTEGER,
    priority INTEGER,
    provider_record_id TEXT,
    zone_id TEXT,
    proxied INTEGER,
    status TEXT,
    remark TEXT,
    extra_data TEXT,
    create_time TEXT NOT NULL,
    update_time TEXT NOT NULL,
    last_sync_time TEXT,
    weight INTEGER,
    type INTEGER NOT NULL DEFAULT 1
);
CREATE INDEX idx_provider_type ON dns_record(provider_type);
CREATE INDEX idx_domain_name ON dns_record(domain_name);
CREATE INDEX idx_record_name ON dns_record(record_name);
CREATE INDEX idx_record_type ON dns_record(record_type);

-- entity: EmailBody.java
CREATE TABLE email_body (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email_body_id TEXT NOT NULL UNIQUE,
    current_version BIGINT,
    tenant_name TEXT,
    tenant_email_config_id BIGINT,
    sender_email TEXT,
    title TEXT,
    content TEXT,
    receive_total BIGINT,
    receive_success_total BIGINT,
    receive_fail_total BIGINT,
    create_time TEXT
);

-- entity: EmailReceive.java
CREATE TABLE email_receive (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT NOT NULL,
    name TEXT NOT NULL,
    create_time TEXT,
    update_time TEXT
);

-- entity: EmailSendRecord.java
CREATE TABLE email_send_record (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email_send_record_id TEXT,
    email_body_id TEXT,
    email_send_address TEXT,
    current_version BIGINT,
    tenant_name TEXT,
    email_receive_id BIGINT,
    receive_email_address TEXT,
    send_state INTEGER,
    create_time TEXT
);

-- entity: InstallApp.java
CREATE TABLE install_app (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    unique_id TEXT NOT NULL UNIQUE,
    ip_address TEXT NOT NULL,
    install_time TEXT NOT NULL,
    create_time TEXT,
    update_time TEXT
);

-- entity: InstanceBackUpDetails.java
CREATE TABLE instance_backup_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT,
    instance_id TEXT,
    display_name TEXT,
    shape TEXT,
    state TEXT,
    ocpus INTEGER,
    memory_in_gbs INTEGER,
    boot_volume_size_in_gbs BIGINT,
    public_ips TEXT,
    private_ips TEXT,
    availability_domain TEXT,
    compartment_id TEXT,
    boot_volume_id TEXT,
    remark TEXT,
    boot_volume_name TEXT,
    ipv6_addresses TEXT,
    username TEXT,
    port INTEGER,
    password TEXT,
    processor_description TEXT,
    architecture TEXT,
    cloud_type INTEGER DEFAULT 1,
    sys_image_backup INTEGER DEFAULT 0
);

-- entity: InstanceCloudNetWork.java
CREATE TABLE instance_cloud_networks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id TEXT,
    vcn_id TEXT,
    vcn_name TEXT,
    subnet_id TEXT,
    subnet_name TEXT,
    region TEXT,
    cidr_block TEXT,
    net_work_security_group_id TEXT,
    created_at TEXT,
    updated_at TEXT,
    cloud_type INTEGER DEFAULT 1
);

-- entity: InstanceDetails.java
CREATE TABLE instance_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT,
    instance_id TEXT,
    display_name TEXT,
    shape TEXT,
    state TEXT,
    ocpus INTEGER,
    memory_in_gbs INTEGER,
    boot_volume_size_in_gbs BIGINT,
    public_ips TEXT,
    private_ips TEXT,
    availability_domain TEXT,
    compartment_id TEXT,
    boot_volume_id TEXT,
    remark TEXT,
    boot_volume_name TEXT,
    vpus_per_gb TEXT,
    ipv6_addresses TEXT,
    vnic_ids TEXT,
    username TEXT,
    port INTEGER,
    password TEXT,
    processor_description TEXT,
    architecture TEXT,
    cloud_type INTEGER DEFAULT 1,
    sys_image_backup INTEGER DEFAULT 0,
    conn_time BIGINT NOT NULL DEFAULT 0,
    enable_ping INTEGER NOT NULL DEFAULT 0,
    on_line_enable INTEGER NOT NULL DEFAULT 1,
    last_on_line_enable INTEGER NOT NULL DEFAULT 1,
    offline_notify INTEGER NOT NULL DEFAULT 0,
    resume_notify INTEGER NOT NULL DEFAULT 0,
    monitor_installed INTEGER,
    last_heartbeat TEXT,
    create_time TEXT
);

-- entity: InstanceTraffic.java
CREATE TABLE instance_traffic (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    instance_id TEXT,
    tenant_id BIGINT,
    tenancy TEXT,
    ingress_bytes REAL,
    egress_bytes REAL,
    stats_date TEXT,
    last_updated TEXT,
    region TEXT,
    threshold REAL,
    auto_shutdown INTEGER,
    created_at TEXT,
    cloud_type INTEGER DEFAULT 1,
    alert_sent INTEGER DEFAULT 0
);

-- entity: LoginUser.java
CREATE TABLE login_user (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    username TEXT NOT NULL UNIQUE,
    password TEXT NOT NULL,
    is_first_user INTEGER,
    login_type TEXT NOT NULL,
    external_id TEXT,
    last_login_at TEXT
);

-- entity: Memo.java
CREATE TABLE memos (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    title TEXT NOT NULL,
    content TEXT,
    summary TEXT,
    create_time TEXT,
    update_time TEXT
);

-- entity: Message.java
CREATE TABLE app_message (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    business_id TEXT,
    message_type TEXT,
    read_status INTEGER,
    subject TEXT,
    content_text TEXT,
    update_time TEXT,
    create_time TEXT
);

-- entity: NginxConfig.java
CREATE TABLE nginx_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_name TEXT,
    config_content TEXT,
    is_current INTEGER,
    config_version INTEGER,
    config_status TEXT,
    create_time TEXT,
    update_time TEXT
);

-- entity: OTPKey.java
CREATE TABLE otp_key (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key_name TEXT UNIQUE,
    secret_key TEXT,
    qr_code TEXT,
    issuer TEXT,
    create_time TEXT NOT NULL,
    update_time TEXT NOT NULL
);

-- entity: OciComputerInfo.java
CREATE TABLE oci_computer_info (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    boot_id_str TEXT,
    computer_create_json TEXT,
    tenant_id BIGINT,
    architecture TEXT,
    cloud_type INTEGER DEFAULT 1,
    computer_region TEXT
);

-- entity: OciMultipartUploadRecord.java
CREATE TABLE oci_multipart_upload_record (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT NOT NULL,
    cloud_type INTEGER DEFAULT 1,
    tenancy_ocid TEXT,
    namespace TEXT,
    bucket_name TEXT NOT NULL,
    object_name TEXT NOT NULL,
    upload_id TEXT NOT NULL,
    total_size BIGINT,
    chunk_size BIGINT,
    total_parts INTEGER,
    completed_parts TEXT,
    status TEXT,
    create_time TEXT,
    update_time TEXT
);

-- entity: OpenBootLock.java
CREATE TABLE open_boot_lock (
    task_id TEXT PRIMARY KEY,
    cloud_type INTEGER NOT NULL DEFAULT 1,
    status TEXT NOT NULL,
    ins_id TEXT,
    create_time TEXT
);

-- entity: OtherBootInstance.java
CREATE TABLE other_boot_instance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version BIGINT NOT NULL DEFAULT 0,
    boot_id TEXT,
    tenant_id BIGINT,
    ocpu INTEGER,
    memory INTEGER,
    disk INTEGER,
    instance_count INTEGER,
    status INTEGER,
    architecture TEXT,
    root_password TEXT,
    public_ip TEXT,
    remark TEXT,
    instance_name TEXT,
    zone TEXT,
    created_at TEXT,
    updated_at TEXT,
    cloud_type INTEGER DEFAULT 2
);

-- entity: ProxyConfig.java
CREATE TABLE proxy_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL UNIQUE,
    target_host TEXT NOT NULL,
    target_port INTEGER NOT NULL,
    protocol TEXT,
    enable_ssl INTEGER,
    enable_websocket INTEGER,
    ssl_certificate_id BIGINT,
    config_status TEXT,
    ssl_status TEXT,
    custom_config TEXT,
    remark TEXT,
    load_balance_type TEXT,
    enable_health_check INTEGER,
    health_check_path TEXT,
    health_check_interval INTEGER,
    enable_rate_limit INTEGER,
    rate_limit INTEGER,
    enable_cache INTEGER,
    cache_time INTEGER,
    create_time TEXT,
    update_time TEXT
);

-- entity: RegisterDetail.java
CREATE TABLE register_detail (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_prv_id BIGINT,
    tenant_id TEXT,
    account_type INTEGER,
    plan_type INTEGER,
    register_time TEXT,
    city TEXT,
    country TEXT,
    email_address TEXT,
    first_name TEXT,
    last_name TEXT,
    line1 TEXT,
    postal_code TEXT,
    subscription_plan_number TEXT,
    upgrade_state TEXT,
    created_time TEXT,
    updated_time TEXT,
    cloud_type INTEGER DEFAULT 1
);

-- entity: ServerMetrics.java
CREATE TABLE server_metrics (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id TEXT,
    server_ip TEXT,
    cpu_usage REAL,
    memory_usage REAL,
    disk_usage REAL,
    upload_traffic REAL,
    download_traffic REAL,
    last_connection_time TEXT,
    cpu_cores INTEGER,
    total_memory REAL,
    total_disk REAL,
    total_upload_traffic TEXT,
    total_download_traffic TEXT
);

-- entity: SslCertificate.java
CREATE TABLE ssl_certificate (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    domain TEXT NOT NULL,
    certificate_type TEXT NOT NULL,
    email TEXT,
    validation_method TEXT,
    auto_renew INTEGER,
    certificate_status TEXT,
    issue_date TEXT,
    expire_date TEXT,
    certificate_path TEXT,
    private_key_path TEXT,
    create_time TEXT,
    update_time TEXT,
    dns_provider TEXT
);

-- entity: SystemConfig.java
CREATE TABLE system_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    config_key TEXT UNIQUE,
    config_value TEXT,
    config_enabled INTEGER,
    last_modified TEXT
);

-- entity: SystemKVStore.java
CREATE TABLE system_kv_store (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key_name TEXT NOT NULL UNIQUE,
    local_value TEXT,
    remark TEXT,
    update_time TEXT,
    create_time TEXT
);

-- entity: TelegramUser.java
CREATE TABLE telegram_user (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id BIGINT NOT NULL UNIQUE,
    username TEXT,
    first_name TEXT,
    last_name TEXT,
    is_authorized INTEGER,
    active INTEGER,
    created_at TEXT NOT NULL,
    last_active_at TEXT
);

-- entity: TemInstance.java
CREATE TABLE tem_instance (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenancy TEXT,
    instance_id TEXT,
    public_ip TEXT,
    region TEXT,
    architecture TEXT,
    root_password TEXT,
    clone_boot_volume_id TEXT,
    cloud_type INTEGER DEFAULT 1
);

-- entity: Tenant.java
CREATE TABLE tenant (
    id INTEGER PRIMARY KEY,
    tenant_id TEXT,
    user_name TEXT,
    fingerprint TEXT,
    tenancy TEXT,
    region TEXT,
    key_file TEXT,
    created_at TEXT,
    api_synced INTEGER,
    enable_icmp INTEGER,
    enable_all_protocol INTEGER,
    is_home_region INTEGER,
    paren_id BIGINT,
    tenancy_name TEXT,
    tenancy_des TEXT,
    account_type TEXT,
    cloud_type INTEGER DEFAULT 1,
    region_en TEXT,
    id_str TEXT,
    email_address TEXT,
    email_enable INTEGER,
    transfer_status INTEGER DEFAULT 0,
    transfer_amount TEXT,
    is_active INTEGER DEFAULT 1
);

-- entity: TenantEmailConfig.java
CREATE TABLE tenant_email_config (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT,
    domain_id TEXT,
    domain_name TEXT,
    sender_id TEXT,
    credential_id TEXT,
    smtp_username TEXT,
    smtp_password TEXT,
    smtp_host TEXT,
    smtp_port TEXT,
    sender_email TEXT,
    dkim_id TEXT,
    cname_record_value TEXT,
    active INTEGER,
    created_time TEXT,
    daily_email_limit BIGINT,
    today_sent_count BIGINT,
    last_reset_date TEXT,
    dbs_record_ids_str TEXT
);

-- entity: TenantSocial.java
CREATE TABLE tenant_social (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT,
    tenancy TEXT,
    cloud_type INTEGER DEFAULT 1,
    client_id TEXT,
    client_secret TEXT,
    social_type_str TEXT,
    third_login_address TEXT,
    redirect_url TEXT,
    social_status TEXT
);

-- entity: TrafficAlert.java
CREATE TABLE traffic_alert (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id BIGINT NOT NULL,
    tenancy TEXT NOT NULL,
    threshold REAL NOT NULL,
    auto_shutdown INTEGER NOT NULL,
    notification_email TEXT,
    enabled INTEGER NOT NULL,
    last_notification TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT,
    statistics_enabled INTEGER NOT NULL,
    cloud_type INTEGER DEFAULT 1
);

-- entity: VpnProxyRecord.java
CREATE TABLE vpn_proxy_record (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    proxy_type TEXT NOT NULL,
    proxy_host TEXT NOT NULL,
    proxy_port INTEGER NOT NULL,
    proxy_username TEXT,
    proxy_password TEXT,
    available_status INTEGER NOT NULL,
    update_time TEXT,
    create_time TEXT
);

-- entity: VpsMonitor.java
CREATE TABLE vps_monitor (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    version BIGINT NOT NULL DEFAULT 0,
    vps_id TEXT NOT NULL UNIQUE,
    vps_name TEXT,
    provider TEXT,
    instance_type TEXT,
    cpu_cores INTEGER,
    memory_gb INTEGER,
    disk_gb INTEGER,
    bandwidth_mbps INTEGER,
    public_ip TEXT,
    private_ip TEXT,
    port INTEGER,
    region TEXT,
    zone TEXT,
    country TEXT,
    city TEXT,
    latitude REAL,
    longitude REAL,
    cpu_usage REAL,
    memory_usage REAL,
    disk_usage REAL,
    network_in_mbps REAL,
    network_out_mbps REAL,
    load_average REAL,
    uptime_hours BIGINT,
    status INTEGER,
    last_ping_time TEXT,
    response_time_ms INTEGER,
    is_monitoring_enabled INTEGER,
    username TEXT,
    password TEXT,
    ssh_key TEXT,
    os_type TEXT,
    os_version TEXT,
    architecture TEXT,
    monthly_cost REAL,
    remark TEXT,
    tags TEXT,
    created_at TEXT,
    updated_at TEXT,
    last_monitor_time TEXT
);
