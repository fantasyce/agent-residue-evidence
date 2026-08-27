//go:build windows

package scope

// Windows-specific reparse-point traversal hardening is added with the native
// file-identity adapter. The common guard already rejects drive roots and a
// reparse point used as the declared root.
