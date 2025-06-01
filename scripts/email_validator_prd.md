# Email Validator Service PRD

<context>
# Overview
The Email Validator Service is a microservice designed to verify email address ownership through a validation process. It provides a reliable way for applications to confirm that users have access to the email addresses they provide during registration or profile updates. This service helps prevent fraud, reduce spam, and ensure data quality by confirming email ownership before granting full access to application features.

# Core Features
## Email Validation Request
- **What it does**: Accepts requests to validate an email address
- **Why it's important**: Initiates the validation workflow and stores the pending validation
- **How it works**: Receives the email address via gRPC or MCP, generates a unique token/code, and triggers an email sending process

## Email Delivery
- **What it does**: Sends validation emails containing either a clickable link or a verification code
- **Why it's important**: Provides the user with a means to prove email ownership
- **How it works**: Uses SMTP or email service APIs to deliver customizable email templates with embedded validation tokens

## Validation Verification
- **What it does**: Verifies user-submitted validation tokens or clicked links
- **Why it's important**: Completes the validation loop and confirms email ownership
- **How it works**: Matches submitted tokens against stored pending validations and marks emails as verified upon success

## Validation Status Checking
- **What it does**: Allows clients to check the current validation status of an email
- **Why it's important**: Enables applications to gate features based on verification status
- **How it works**: Provides a simple API to query whether an email has been verified

# User Experience
## User Personas
1. **Application Developers**: Engineers integrating email validation into their applications
2. **End Users**: People receiving validation emails and completing the verification process

## Key User Flows
1. **Developer Integration Flow**:
   - Developer configures service connection
   - Application requests email validation
   - Application checks validation status
   - Application receives validation results

2. **End User Validation Flow**:
   - User receives validation email
   - User either clicks the validation link or copies the verification code
   - User submits the code or is redirected to a confirmation page
   - User receives confirmation of successful validation

## UI/UX Considerations
- Email templates should be clean, professional, and customizable
- Validation links should be clearly visible and trustworthy
- Verification codes should be easy to read and copy
- Confirmation pages should provide clear feedback on validation status
</context>

<PRD>
# Technical Architecture

## System Components
1. **API Service**
   - gRPC server implementing the validation service interface
   - MCP protocol adapter for cloud compatibility
   - Request validation and rate limiting

2. **Email Manager**
   - Email template rendering
   - SMTP client integration
   - Delivery status tracking

3. **Token Manager**
   - Secure token generation
   - Token storage and retrieval
   - Token expiration handling with configurable TTL
   - Automatic invalidation of expired tokens

4. **Validation Store**
   - Database for storing validation records
   - Query interface for validation status
   - Automatic cleanup of expired validation attempts
   - Expiration status tracking and reporting

5. **Webhook Handler**
   - Callback processing for clicked links
   - Validation confirmation

## Protocol Buffer Design Guidelines

The following guidelines should be followed when implementing the Protocol Buffer definitions for this service:

1. **Clear Field Documentation**
   - Every field must have a descriptive comment explaining its purpose, constraints, and behavior
   - Include examples for complex or non-obvious fields
   - Document default values and behavior when fields are not provided

2. **Prefer Enums Over Booleans**
   - Use enums instead of booleans when a field might have more than two states in the future
   - Always include an UNSPECIFIED (0) value for each enum
   - Follow the naming convention: ENUM_TYPE_VALUE_NAME

3. **Structured Message Design**
   - Use nested messages for logically grouped fields
   - Avoid flat structures with many fields at the top level
   - Create dedicated message types for reusable field groups

4. **Future Extensibility**
   - Use oneof for mutually exclusive fields that might expand
   - Design with multi-channel verification in mind (email, phone, etc.)
   - Plan for backward compatibility in future versions

5. **Field Numbering**
   - Use field numbers 1-15 for frequently used fields (1-byte encoding)
   - Choose field numbers carefully for optimal wire format efficiency
   - Maintain consistent numbering patterns across similar messages

6. **Versioning Strategy**
   - Follow backward compatibility best practices
   - Use optional fields for all new additions
   - Never change field numbers or types

## Data Models

### Contact Information
```
// ContactInfo represents a contact method that can be validated
message ContactInfo {
  // Types of contact methods supported by the system
  enum Type {
    // Unknown or unspecified contact type
    CONTACT_TYPE_UNSPECIFIED = 0;

    // Email address validation
    EMAIL = 1;

    // Reserved for future phone number validation
    // PHONE = 2;
  }

  // The type of contact information provided
  Type type = 1;

  // Contact information based on type
  oneof contact {
    // Email address to validate when type is EMAIL
    string email = 2;

    // Reserved for future phone number validation
    // PhoneNumber phone = 3;
  }
}

// PhoneNumber is reserved for future implementation
// message PhoneNumber {
//   string country_code = 1;
//   string number = 2;
// }
```

### ValidationRequest
```
// ValidationRequest initiates the validation of a contact method
message ValidationRequest {
  // Contact information to validate (email, phone in future)
  ContactInfo contact_info = 1;

  // Configuration for the validation process
  ValidationConfig config = 2;

  // Optional callback URL to notify when validation completes
  string callback_url = 3;

  // Optional client-provided metadata for tracking or customization
  map<string, string> metadata = 4;
}

// ValidationConfig contains settings for the validation process
message ValidationConfig {
  // Method to use for validation
  ValidationMethod method = 1;

  // Custom expiration duration, defaults to 24h if not specified
  google.protobuf.Duration expiration = 2;

  // Optional template customization parameters
  TemplateOptions template_options = 3;
}

// TemplateOptions allows customization of validation emails/messages
message TemplateOptions {
  // Optional custom subject line for email
  string subject = 1;

  // Optional sender name to display
  string sender_name = 2;

  // Optional template ID to use instead of default
  string template_id = 3;

  // Optional custom variables to include in template
  map<string, string> variables = 4;
}

// ValidationMethod defines how the validation will be performed
enum ValidationMethod {
  // Unknown or unspecified validation method
  VALIDATION_METHOD_UNSPECIFIED = 0;

  // Validation via clickable link in email/message
  LINK = 1;

  // Validation via code entry
  CODE = 2;
}
```

### ValidationRecord
```
// ValidationRecord represents a validation attempt in the system
message ValidationRecord {
  // Unique identifier for this validation record
  string id = 1;

  // Contact information being validated
  ContactInfo contact_info = 2;

  // Validation token (link token or verification code)
  string token = 3;

  // Method used for this validation
  ValidationMethod method = 4;

  // Current status of the validation
  ValidationStatus status = 5;

  // Timestamps for tracking the validation lifecycle
  ValidationTimestamps timestamps = 6;

  // Client-provided metadata from the original request
  map<string, string> metadata = 7;

  // Number of verification attempts made
  int32 attempt_count = 8;
}

// ValidationTimestamps tracks important times in the validation lifecycle
message ValidationTimestamps {
  // When the validation was initially requested
  google.protobuf.Timestamp created_at = 1;

  // When the validation will expire
  google.protobuf.Timestamp expires_at = 2;

  // When the validation was successfully completed (if applicable)
  google.protobuf.Timestamp validated_at = 3;

  // When the most recent verification attempt occurred
  google.protobuf.Timestamp last_attempt_at = 4;
}

// ValidationStatus represents the current state of a validation
enum ValidationStatus {
  // Unknown or unspecified status
  VALIDATION_STATUS_UNSPECIFIED = 0;

  // Validation has been created but not yet completed
  PENDING = 1;

  // Validation was successfully completed
  VALIDATED = 2;

  // Validation expired before completion
  EXPIRED = 3;

  // Validation failed (too many attempts, etc.)
  FAILED = 4;

  // Validation was canceled by the requestor
  CANCELED = 5;
}
```

### StatusRequest
```
// StatusRequest retrieves the current status of a validation
message StatusRequest {
  // One of the following must be provided
  oneof identifier {
    // The validation record ID
    string validation_id = 1;

    // The contact information to check status for
    ContactInfo contact_info = 2;
  }
}
```

### StatusResponse
```
// StatusResponse provides the current status of a validation
message StatusResponse {
  // Current status of the validation
  ValidationStatus status = 1;

  // Validation record ID
  string validation_id = 2;

  // Contact information being validated
  ContactInfo contact_info = 3;

  // Timestamps for the validation
  ValidationTimestamps timestamps = 4;
}
```

### VerifyCodeRequest
```
// VerifyCodeRequest submits a verification code for validation
message VerifyCodeRequest {
  // One of the following must be provided to identify the validation
  oneof identifier {
    // The validation record ID
    string validation_id = 1;

    // The contact information being validated
    ContactInfo contact_info = 2;
  }

  // The verification code to check
  string code = 3;
}
```

### CancelRequest
```
// CancelRequest cancels a pending validation
message CancelRequest {
  // One of the following must be provided
  oneof identifier {
    // The validation record ID
    string validation_id = 1;

    // The contact information to cancel validation for
    ContactInfo contact_info = 2;
  }

  // Optional reason for cancellation
  string reason = 3;
}
```

### ExtendExpirationRequest
```
// ExtendExpirationRequest extends the expiration time of a pending validation
message ExtendExpirationRequest {
  // One of the following must be provided
  oneof identifier {
    // The validation record ID
    string validation_id = 1;

    // The contact information to extend validation for
    ContactInfo contact_info = 2;
  }

  // Additional time to extend the expiration by
  google.protobuf.Duration extension = 3;
}
```

## APIs and Integrations

### gRPC Service Definition
```
service EmailValidator {
  // Initiates the validation process for a contact method (currently email only)
  // Returns a validation record with a unique ID and token
  rpc RequestValidation(ValidationRequest) returns (ValidationRecord);

  // Retrieves the current status of a validation request
  // Can be queried by validation ID or contact information
  rpc CheckStatus(StatusRequest) returns (StatusResponse);

  // Verifies a code submitted by the user (for CODE validation method)
  // Returns the updated status of the validation
  rpc VerifyCode(VerifyCodeRequest) returns (StatusResponse);

  // Cancels a pending validation request
  // No effect if validation is already completed or expired
  rpc CancelValidation(CancelRequest) returns (google.protobuf.Empty);

  // Extends the expiration time of a pending validation
  // Returns the updated validation record with new expiration time
  rpc ExtendExpiration(ExtendExpirationRequest) returns (ValidationRecord);
}
```

### MCP Protocol Mapping
- Map gRPC service to MCP resources and operations
- Support for MCP discovery and health checking
- Compatible with Google Cloud Run and similar platforms

### External Integrations
- SMTP server or email service API (SendGrid, Mailgun, etc.)
- Database storage (Cloud SQL, Firestore, etc.)
- Monitoring and logging services

## Technical Architecture

1. **Build System**
   - Bazel for reproducible, hermetic builds
   - Centralized dependency management
   - Fast, incremental builds with caching
   - Cross-platform compatibility
   - Integrated test execution framework

2. **Development Practices**
   - Test-driven development (TDD) methodology
   - Write tests before implementation code
   - Maintain high test coverage
   - Automated testing in CI pipeline
   - Regular test execution during development

3. **Deployment Environment**
   - Google Cloud Run for containerized services
   - Cloud SQL for persistent storage
   - Secret Manager for credentials and sensitive configuration
   - Redis for rate limiting and temporary token storage
   - Load balancer for traffic distribution
   - Monitoring and alerting setup
   - Independent service modules with well-defined interfaces
   - Observability stack (metrics, logging, tracing)

## Modular Design Principles

### UNIX Philosophy
- Each component should do one thing and do it well
- Components should work together through well-defined interfaces
- Components should be independently testable and deployable
- Design for composition rather than monolithic implementation

### Module Independence
1. **API Service Module**
   - Can run with mock implementations of other modules
   - Provides simulation mode for testing without external dependencies
   - Configurable to use different implementations of other modules

2. **Email Delivery Module**
   - Can be tested independently with mock email targets
   - Supports local development mode that logs emails instead of sending
   - Pluggable provider system (SMTP, SendGrid, AWS SES, etc.)

3. **Token Management Module**
   - Standalone token generation and validation
   - Can use different storage backends (memory, Redis, database)
   - Provides CLI tools for token debugging and management

4. **Validation Store Module**
   - Abstracted storage interface with multiple implementations
   - In-memory implementation for testing
   - Database implementation for production
   - Migration tools for schema evolution

### Interface Contracts
- Clear API contracts between modules using Protocol Buffers
- Versioned interfaces to support gradual evolution
- Comprehensive interface documentation
- Automated contract testing between modules

## Observability Engineering

### Metrics Collection
- Request rate, latency, and error metrics for all API endpoints
- Email delivery success/failure rates and latency
- Token creation and validation metrics
- Resource utilization metrics (CPU, memory, connections)
- Business metrics (validation success rate, expiration rate)

### Structured Logging
- Consistent log format across all modules
- Correlation IDs to track requests across components
- Log levels configurable at runtime
- Sensitive data redaction in logs
- Log aggregation and search capabilities

### Distributed Tracing
- OpenTelemetry integration for end-to-end request tracing
- Span propagation across module boundaries
- Latency breakdown by component
- Error attribution to specific modules

### Health Monitoring
- Readiness and liveness probes for each module
- Detailed health check endpoints with component status
- Synthetic monitoring for end-to-end validation
- Alerting on key performance and reliability indicators

# Development Roadmap

## Phase 1: MVP
- Set up Bazel build system with initial WORKSPACE and BUILD files
- Implement test-driven development workflow with initial test suites
- Basic gRPC service implementation with link-based validation
- Simple email delivery using SMTP
- In-memory storage for validation records
- Token expiration and automatic cleanup mechanism
- Minimal error handling and logging
- Basic documentation for integration
- Modular design with clear interfaces between components
- Basic observability (metrics and logging)

## Phase 2: Core Features Enhancement
- Add code-based validation method
- Implement persistent storage with database
- Add rate limiting and abuse prevention
- Enhance error handling and reporting
- Improve email templates and customization
- Enhance observability with distributed tracing
- Add mock implementations for all modules
- Create CLI tools for independent module testing

## Phase 3: Cloud Integration
- Implement MCP protocol adapter
- Configure cloud deployment (Google Cloud Run)
- Set up comprehensive monitoring and alerting
- Add horizontal scaling capabilities
- Implement proper secrets management
- Deploy modules as independent services
- Implement service discovery and configuration management

## Phase 4: Advanced Features
- Add webhook callbacks for validation events
- Implement email template customization API
- Add multi-tenancy support
- Enhance security features (IP tracking, suspicious activity detection)
- Implement comprehensive metrics and dashboards

## Phase 5: Enterprise Features
- Add audit logging
- Implement compliance features (GDPR, etc.)
- Add advanced analytics
- Support for custom domains
- SLA monitoring and reporting

# Logical Dependency Chain

1. **Foundation Layer**
   - gRPC service definition and implementation
   - Basic validation record storage
   - Simple token generation and verification

2. **Email Functionality**
   - Email template creation
   - SMTP integration
   - Link generation and handling

3. **Validation Logic**
   - Link validation flow
   - Code validation flow
   - Status checking and reporting

4. **Persistence Layer**
   - Database schema design
   - Data access layer implementation
   - Migration from in-memory to persistent storage

5. **Cloud Deployment**
   - Containerization
   - MCP protocol adapter
   - Cloud service configuration

6. **Advanced Features**
   - Rate limiting
   - Webhook support
   - Enhanced security features

# Risks and Mitigations

## Technical Challenges
- **Email Deliverability**: Implement proper SPF, DKIM, and DMARC records; use established email delivery services
- **Security Vulnerabilities**: Regular security audits, token encryption, rate limiting
- **Scalability Issues**: Design for horizontal scaling, use cloud-native services, implement caching

## MVP Risks
- **Scope Creep**: Strictly define MVP features and resist adding complexity until core functionality is solid
- **Integration Complexity**: Provide clear documentation and examples for developers
- **Performance Bottlenecks**: Start with simple architecture and optimize based on real usage patterns

## Resource Constraints
- **Development Time**: Focus on modular architecture to allow parallel development
- **Testing Complexity**: Implement comprehensive automated testing, especially for email delivery
- **Operational Overhead**: Use managed services where possible to reduce maintenance burden

# Appendix

## Email Template Examples
```html
<!-- Link-based validation email -->
<div>
  <h2>Verify Your Email Address</h2>
  <p>Please click the button below to verify your email address:</p>
  <a href="{{validation_link}}" style="display: inline-block; padding: 10px 20px; background-color: #4CAF50; color: white; text-decoration: none; border-radius: 5px;">Verify Email</a>
  <p>Or copy and paste this link in your browser:</p>
  <p>{{validation_link}}</p>
  <p>This link will expire in 24 hours.</p>
</div>

<!-- Code-based validation email -->
<div>
  <h2>Verify Your Email Address</h2>
  <p>Your verification code is:</p>
  <div style="font-size: 24px; font-weight: bold; padding: 10px; background-color: #f0f0f0; border-radius: 5px; letter-spacing: 5px;">{{verification_code}}</div>
  <p>This code will expire in 24 hours.</p>
</div>
```

## Performance Benchmarks
- Target response time: < 200ms for API calls
- Email delivery time: < 2 minutes for 99% of emails
- System capacity: 100 requests per second per instance
- Validation success rate: > 95% for valid emails

## Security Considerations
- Token encryption using industry-standard algorithms
- Short-lived tokens with configurable expiration (default 24 hours)
- Rate limiting by IP and email domain
- Protection against email enumeration attacks
- Regular security audits and penetration testing
- Proper field validation to prevent injection attacks
- Secure storage of validation records with appropriate access controls
- Forward compatibility for adding additional verification channels
- Security-focused observability (audit logging, anomaly detection)
- Independent security testing of each module
</PRD>
