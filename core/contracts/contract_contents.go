package contracts

import _ "embed"

//go:embed primitive_stream.kf
var PrivateContractContent []byte

//go:embed composed_stream_template.kf
var ComposedContractContent []byte

//go:embed primitive_stream_unix.kf
var PrivateUnixContractContent []byte

//go:embed composed_stream_template_unix.kf
var ComposedUnixContractContent []byte

//go:embed helper_stream.kf
var HelperContractContent []byte
