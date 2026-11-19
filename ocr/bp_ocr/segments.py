"""Seven segment order: top, upper right, lower right, bottom, lower left, upper left, middle."""

import numpy as np

CODES = (
    "1111110",
    "0110000",
    "1101101",
    "1111001",
    "0110011",
    "1011011",
    "1011111",
    "1110000",
    "1111111",
    "1111011",
)
PATTERNS = [(digit, code) for digit, code in enumerate(CODES)]
PATTERNS.append((7, "1110010"))  # The HEM-1026 lights the upper left bar of 7 too.


def candidates(states, allow_blank=False):
    matches = {
        digit
        for digit, code in PATTERNS
        if all(s is None or int(c) == s for s, c in zip(states, code))
    }
    if allow_blank and 1 not in states:
        matches.add("")  # A hidden leading 1 must remain distinguishable from a blank.
    return matches


# Segment centres in fractions of the digit cell.
CENTERS = np.float32(
    [
        [0.50, 0.062],
        [0.89, 0.265],
        [0.89, 0.735],
        [0.50, 0.938],
        [0.11, 0.735],
        [0.11, 0.265],
        [0.50, 0.50],
    ]
)
HORIZONTAL = (0, 3, 6)
