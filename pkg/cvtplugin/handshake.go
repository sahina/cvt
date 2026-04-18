package cvtplugin

import (
	"github.com/hashicorp/go-plugin"
)

// ProtocolVersion is the CVT plugin protocol version. Bumped on breaking
// changes to the plugin ABI. Core and plugin must agree.
const ProtocolVersion uint32 = 1

// Handshake is the hashicorp/go-plugin handshake shared between CVT core
// and every CVT plugin. The magic cookie is a handshake-only check (NOT
// authentication); a mismatched cookie means the plugin binary was
// invoked directly by a user rather than launched by CVT, and the plugin
// exits with a helpful message.
var Handshake = plugin.HandshakeConfig{
	ProtocolVersion:  uint(ProtocolVersion),
	MagicCookieKey:   "CVT_PLUGIN_MAGIC_COOKIE",
	MagicCookieValue: "cvt-plugin-v1",
}

// Service identifiers declared in PluginHandshake.Info.services.
const (
	ServiceRegistryV1 = "registry.v1"
	ServiceEventsV1   = "events.v1"
)
