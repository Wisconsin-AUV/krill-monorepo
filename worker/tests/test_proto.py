from krill.v1 import health_pb2


def test_generated_proto_roundtrip() -> None:
    msg = health_pb2.CheckResponse(version="v1.2.3")
    parsed = health_pb2.CheckResponse.FromString(msg.SerializeToString())
    assert parsed.version == "v1.2.3"
