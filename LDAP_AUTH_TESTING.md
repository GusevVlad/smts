# LDAP Authorization Testing Guide

This guide explains how to test the SMTS LDAP authorization system using two complementary test scripts.

## Overview

The SMTS system now implements role-based LDAP authorization with two test scripts:

1. **`test-message-flow-ldap.sh`** - Tests successful message flows with users having proper roles
2. **`test-permission-failures.sh`** - Tests authorization failures with users without proper roles

## Test Users Configuration

### Environment Setup

Both scripts use environment variables from `tests/examples/.env`:

```bash
# Test user WITH proper roles (for successful tests)
LDAP_USERNAME=testuser
LDAP_PASSWORD=testpass

# Test user WITHOUT proper roles (for failure tests)
LDAP_USERNAME_NO_ROLES=norolesuser
LDAP_PASSWORD_NO_ROLES=norolespass
```

### LDAP Server Requirements

For proper testing, your LDAP server should have:

1. **`testuser`** - Should be in one or more of these groups:
   - `testuser` (defined in ldap-roles.yaml)
   - `admin` (has all permissions)
   - `ext_writer`, `ext_reader`, `int_writer`, `int_reader`
   - `appsec_writer`, `appsec_reader`, `dos_writer`, `dos_reader`

2. **`norolesuser`** - Should exist in LDAP but **NOT** be in any of the required groups

## Running the Tests

### 1. Test Successful Authorization

```bash
./test-message-flow-ldap.sh
```

**Expected Results:**
- ✅ All endpoints return 200 OK
- ✅ Messages flow successfully between EXT and INT
- ✅ LDAP authentication works correctly

**What this verifies:**
- Users with proper roles can access all endpoints
- Message flow works end-to-end
- LDAP authentication is functioning

### 2. Test Authorization Failures

```bash
./test-permission-failures.sh
```

**Expected Results:**
- ✅ All endpoints return 403 Forbidden
- ✅ Authorization correctly denies access
- ✅ LDAP authentication still works (user exists)

**What this verifies:**
- Users without proper roles are denied access
- Authorization system is working correctly
- 403 responses are returned for insufficient permissions

## Test Coverage

### Endpoints Tested

Both scripts test all new REST API endpoints:

- **`/send`** (POST) - Send messages
- **`/receive`** (GET) - Receive messages  
- **`/processed`** (POST) - Confirm message processing

### Authorization Scenarios

| Scenario | User | Expected Result | Script |
|----------|------|-----------------|---------|
| Successful access | `testuser` (with roles) | 200 OK | `test-message-flow-ldap.sh` |
| Authorization failure | `norolesuser` (no roles) | 403 Forbidden | `test-permission-failures.sh` |
| Authentication failure | Invalid credentials | 401 Unauthorized | Both scripts |

## LDAP Roles Configuration

The authorization system uses `configs/ldap-roles.yaml` which defines:

- **Roles** with specific permissions
- **Endpoint mappings** for different deployments (EXT/INT)
- **Required permissions** for each endpoint type

### Example Role Configuration

```yaml
roles:
  testuser:
    description: "Test user with full access"
    permissions: ["ext:send", "ext:read", "int:send", "int:read"]
  
  appsec_writer:
    description: "Application security team - write access"
    permissions: ["ext:send", "int:send"]
  
  dos_writer:
    description: "DOS monitoring team - write access"
    permissions: ["ext:send", "int:send"]
```

## Troubleshooting

### Common Issues

1. **401 Unauthorized**
   - User doesn't exist in LDAP
   - Wrong password
   - LDAP server not accessible

2. **403 Forbidden** (when expecting 200)
   - User not in required LDAP groups
   - YAML configuration not loaded
   - Fallback hardcoded groups don't include user

3. **200 OK** (when expecting 403)
   - User has permissions they shouldn't have
   - Fallback mechanism providing access
   - YAML configuration not being used

### Debug Steps

1. Check LDAP server logs for authentication attempts
2. Verify user group memberships in LDAP
3. Check SMTS logs for authorization decisions
4. Ensure `ldap-roles.yaml` is properly formatted and accessible
5. Verify environment variables are set correctly

## Integration with CI/CD

Both scripts can be integrated into your CI/CD pipeline:

```yaml
# Example GitHub Actions workflow
jobs:
  test-ldap-auth:
    runs-on: ubuntu-latest
    steps:
      - name: Test successful authorization
        run: ./test-message-flow-ldap.sh
      
      - name: Test authorization failures
        run: ./test-permission-failures.sh
```

## Security Considerations

- Never commit real LDAP credentials to version control
- Use separate test users for development and production
- Regularly audit LDAP group memberships
- Monitor authorization logs for suspicious activity
- Keep YAML configuration files secure

## Related Files

- `configs/ldap-roles.yaml` - Role-based permission configuration
- `internal/server/ldap_middleware.go` - LDAP authentication middleware
- `internal/server/server.go` - REST API endpoint implementations
- `tests/examples/.env` - Test environment configuration