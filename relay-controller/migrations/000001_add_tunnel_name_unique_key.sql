ALTER TABLE tunnel
    ADD UNIQUE KEY IF NOT EXISTS uk_tunnel_namespace_name (namespace, name);
