package com.huawei.devbridge.relaycontroller.infrastructure.persistence.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableName;
import com.huawei.devbridge.relaycontroller.domain.model.TunnelType;
import lombok.Data;

@Data
@TableName("tunnel")
public class TunnelEntity {
    @TableId(value = "_id", type = IdType.AUTO)
    private Long id;
    private String name;
    private String tunnelId;
    private Long tunnelCode;
    private String clusterId;
    private Integer expiration;
    private Integer expirationHours;
    private String namespace;
    private Long accountId;
    private String description;
    private Long bandwidthUsed;
    private String url;
    private TunnelType type;
    private Integer deleted;
    private Long createdAt;
    private Long updatedAt;
    @TableField(exist = false)
    private Long portCount;
}
