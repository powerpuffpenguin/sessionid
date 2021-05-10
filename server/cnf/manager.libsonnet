local def = import "def.libsonnet";
local method = def.Method;
local coder = def.Coder;
{
    // Token signature algorithm
    Method: method.HMD5,
    // Signing key
    Key: 'cerberus is an idea',
    // Serialization coder
    Coder: coder.JSON,
}