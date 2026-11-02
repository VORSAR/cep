"""A stream of possibly multi-line JSON objects, as `go vet -json` and `go list -json` print them."""

import json


def parse(data: bytes) -> list[dict]:
    """Every top-level object, in order. Anything else in the stream raises ValueError."""
    text = data.decode("utf-8", errors="replace")
    decoder = json.JSONDecoder()
    objects: list[dict] = []
    i = 0
    while True:
        while i < len(text) and text[i].isspace():
            i += 1
        if i == len(text):
            return objects
        obj, i = decoder.raw_decode(text, i)
        if not isinstance(obj, dict):
            raise ValueError(f"expected a JSON object, got {type(obj).__name__}")
        objects.append(obj)
