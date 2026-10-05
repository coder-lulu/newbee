# NewBee CMDB Service Analysis

## 1. Overview

The **NewBee CMDB** (Configuration Management Database) is a core component of the NewBee system, designed to manage IT infrastructure configuration items (CIs) and their relationships. It utilizes a dynamic **Entity-Attribute-Value (EAV)** architecture to support flexible, user-defined data models without requiring database schema changes.

The service is built using the **go-zero** framework and consists of two main microservices:
*   **`cmdb-api`**: The HTTP Gateway exposing RESTful APIs.
*   **`cmdb-rpc`**: The gRPC service handling core business logic and database interactions.

## 2. Architecture & Technologies

| Component | Technology | Description |
| :--- | :--- | :--- |
| **Framework** | go-zero | High-performance microservices framework. |
| **ORM** | Ent | Entity framework for Go, handling the graph-based data model. |
| **Database** | MySQL | Primary storage for CIs, Types, and Relations. |
| **Cache** | Redis | Caching for high-read performance. |
| **Protocol** | gRPC / Protobuf | Internal communication between API and RPC layers. |
| **Security** | Casbin | Policy-based access control and data permissions. |

## 3. Data Model (Dynamic EAV)

The CMDB does not map one CI type to one database table. Instead, it uses a metadata-driven approach:

### 3.1 Core Entities

*   **`CiType` (`cmdb_ci_types`)**: Defines a class of CIs (e.g., "Server", "Switch", "Database").
    *   Supports inheritance (`CiTypeInheritance`) allows types to inherit attributes from parent types.
*   **`Attribute` (`cmdb_attributes`)**: Defines a property of a `CiType` (e.g., "IP Address", "Hostname", "CPU Cores").
    *   Includes metadata for validation, display types (text, int, choice), and uniqueness.
*   **`Cis` (`cmdb_cis`)**: The actual instances of configuration items.
    *   Does **not** store attribute values directly.
    *   Links to `CiType` for definition.

### 3.2 Value Storage

Attribute values are stored in type-specific tables to maintain data integrity and query performance:

*   `ValueText`
*   `ValueInteger`
*   `ValueFloat`
*   `ValueDatetime`
*   `ValueJson`

A `Cis` record links to these value records based on the attributes defined in its `CiType`.

### 3.3 Relationships

*   **`CiRelation`**: Defines a directed link between two CIs (Source -> Target).
*   **`RelationType`**: Defines the semantic meaning of a relationship (e.g., "Hosted On", "Connected To").
*   **Topology**: The system can build full topology graphs by traversing these relations.

## 4. Key Features

### 4.1 Dynamic Schema Management
Users can create new CI Types and Attributes at runtime via the API. The system automatically handles the metadata, allowing immediate creation of CIs of the new type without code deployment.

### 4.2 Advanced Search & Filtering
The API supports a complex query language (`CisListReq`) including:
*   **Attribute Filters**: Filter by specific dynamic attributes.
*   **Relation Filters**: Find CIs connected to specific targets or types.
*   **Time Travel**: Query historical states (supported by audit fields).
*   **Full-text Search**: Across text attributes.

### 4.3 Auto-Discovery & Integration
*   **`CiTypeDiscoveryConfig`**: Configures how external data is mapped to CIs.
*   **Integration**: Supports "push" (API) and "pull" (Agent/Scanner) models.
*   **Conflict Resolution**: Configurable strategies (merge, overwrite) when discovered data conflicts with existing data.

### 4.4 Multi-Tenancy & Security
*   **Tenant Isolation**: Strict data isolation enforced via `TenantMixin` and `TenantCheck` middleware.
*   **Data Permissions**: Fine-grained access control (Row/Column level) using Casbin and `DataPerm` middleware.
*   **Audit Logging**: All mutations are tracked.

## 5. API & Interface

### 5.1 HTTP API (`cmdb-api`)
*   **Port**: 9201 (default)
*   **Key Endpoints**:
    *   `POST /api/v1/cmdb/cis/list`: Advanced search for CIs.
    *   `POST /api/v1/cmdb/cis/create`: Create a new CI.
    *   `POST /api/v1/cmdb/ci_type/create`: Define a new CI Type.
    *   `POST /api/v1/cmdb/cis/relations`: Query topology.

### 5.2 RPC Interface (`cmdb-rpc`)
*   **Port**: 9200 (default)
*   **Service**: `Cmdb`
*   Used by other internal services (like `Core`) to query infrastructure data.

## 6. Configuration

Configuration is managed via YAML files in `etc/`.

*   **Database**: Configured in `DatabaseConf` (MySQL).
*   **Middleware**:
    *   `TenantCheck`: Enabled by default for multi-tenancy.
    *   `DataPerm`: Enabled for permission scope enforcement.
*   **Adapters**: configuration for Excel import and API batch processing limits.

## 7. Project Structure

```
cmdb/
├── api/             # HTTP Gateway
│   ├── desc/        # API definitions (.api files)
│   ├── etc/         # Configuration
│   └── internal/    # API Handlers and Logic
├── rpc/             # gRPC Service
│   ├── cmdb.proto   # Protobuf definition
│   ├── ent/         # Ent ORM generated code & schema
│   │   └── schema/  # Data Model definitions
│   ├── etc/         # Configuration
│   └── internal/    # Business Logic
```

## 8. Compliance & Standards

The service strictly adheres to the **NewBee Coding Guidelines**:
*   **Mixins**: Uses `TenantMixin`, `DepartmentMixin`, `StatusMixin` from `newbee-common`.
*   **Layering**: Clear separation between Transport (API/RPC) and Domain Logic.
*   **Safety**: No raw SQL; all database access via Ent ORM with Hooks for tenant isolation.
