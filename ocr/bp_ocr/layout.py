"""Right-aligned digit cells on the Chinese HEM-1026, in 960 × 480 LCD coordinates."""

import cv2
import numpy as np

from .segments import CENTERS, HORIZONTAL, candidates

# Right edge, top, cell width, cell height and cell pitch.
FIELDS = {
    "systolic": (630, 113, 94, 155, 124),
    "diastolic": (630, 300, 94, 155, 124),
    "pulse": (933, 351, 64, 104, 82),
}


def contrast_maps(gray, width, height):
    # Opposite sides of a bar are normally both brighter. A one-sided
    # brightness step at the rim of a reflection should have little response.
    maps = []
    for horizontal in (True, False):
        if horizontal:
            block, axis, offset = (
                (round(width * 0.44), round(height * 0.044)),
                0,
                round(height * 0.12),
            )
        else:
            block, axis, offset = (
                (round(width * 0.072), round(height * 0.22)),
                1,
                round(width * 0.24),
            )
        core = cv2.blur(gray.astype(np.float32), block)
        a = np.roll(core, offset, axis=axis) - core
        b = np.roll(core, -offset, axis=axis) - core
        maps.append(np.minimum(a, b) * 0.75 + np.maximum(a, b) * 0.25)
    return maps


def register(maps, spec):
    """Project responses from the lit strokes onto the possible cell positions."""
    right, top, width, height, pitch = spec
    # A partly hidden rim principally changes the horizontal scale of the warp.
    scales, dx, dy, tilt = np.meshgrid(
        np.arange(0.90, 1.061, 0.04),
        np.arange(-101, 29, 3),
        np.arange(-15, 16, 3),
        [-0.12, -0.06, 0.0, 0.06, 0.12],
        indexing="ij",
    )
    edge, y0 = right + dx, top + dy
    score = np.zeros(edge.shape)
    for slot in (0, 1, 2):
        for j, (sx, sy) in enumerate(CENTERS):
            if slot == 2 and j not in (1, 2):
                continue
            x = np.rint(
                edge
                - ((1 - sx) * width + slot * pitch) * scales
                + (sy - 0.5) * height * tilt
            ).astype(int)
            y = np.rint(y0 + sy * height).astype(int)
            response = maps[0 if j in HORIZONTAL else 1][y, x]
            score += 32 * np.log1p(np.clip(response, 0, 170) / 24)
    score -= 0.15 * abs(dx) + 0.32 * abs(dy) + 20 * abs(scales - 1) + 12 * abs(tilt)
    best = np.unravel_index(score.argmax(), score.shape)
    return edge[best], y0[best], scales[best], tilt[best]


def sample_bar(gray, unknown, maps, j, x, y, w, height):
    horizontal = j in HORIZONTAL
    half_x = round(w * (0.21 if horizontal else 0.038))
    half_y = round(height * (0.022 if horizontal else 0.145))
    patch = np.s_[y - half_y : y + half_y + 1, x - half_x : x + half_x + 1]
    visible = ~unknown[patch]
    if visible.mean() < 0.55:
        return None, 0.0
    response = maps[0 if horizontal else 1][patch][visible]
    contrast = float(np.percentile(response, 75))
    surround = gray[y - half_y - 8 : y + half_y + 9, x - half_x - 8 : x + half_x + 9]
    low, high = np.percentile(surround, [10, 90])
    glare = high > 190
    glare_edge = np.median(gray[patch]) > 190 and high - low > 12
    off, on = (7.5, 10.0) if glare else (15.0, 22.0)
    if glare_edge:
        off = 3.0
    if contrast >= on:
        return 1, min(1.0, contrast / 45.0)
    if contrast <= off:
        return 0, 1.0
    return None, 0.5


def read_field(gray, normalized, unknown, name):
    spec = FIELDS[name]
    _, _, width, height, pitch = spec
    maps = contrast_maps(normalized, width, height)
    right, top, scale, tilt = register(maps, spec)
    digits, boxes = [], []
    confidence = 1.0
    has_digit = False
    for slot in (2, 1, 0):
        left = right - (width + slot * pitch) * scale
        w = width * scale
        states = [0] * 7
        strengths = []
        for j, (sx, sy) in enumerate(CENTERS):
            # The pulse hundreds cell only has the two bars needed to display 1.
            if name == "pulse" and slot == 2 and j not in (1, 2):
                continue
            x = round(left + sx * w + (sy - 0.5) * height * tilt)
            y = round(top + sy * height)
            state, strength = sample_bar(gray, unknown, maps, j, x, y, w, height)
            states[j] = state
            strengths.append(strength)
        matches = candidates(states, allow_blank=slot > 0 and not digits)
        has_digit |= bool(matches) and 1 in states
        if matches == {""}:
            continue
        boxes.append((round(left), round(top), round(w), height))
        digits.append(matches.pop() if len(matches) == 1 else None)
        confidence = min(confidence, *strengths)
    value = None if None in digits else int("".join(map(str, digits)))
    return value, round(confidence, 2) if value is not None else 0.0, boxes, has_digit


def high_indicator(gray, unknown):
    # Both long horizontal edges of the dark indicator survive even when the
    # warning text is too faint to read. Compare them against the nearby glass.
    region = gray[:, 755:890].astype(np.float32)
    if unknown[165:210, 755:890].mean() > 0.6:
        return None
    profile = np.median(region, axis=1)
    for top in range(151, 182):
        if profile[top - 6] - profile[top + 6] < 6:
            continue
        for height in (42, 46, 50, 54):
            bottom = top + height
            if profile[bottom + 6] - profile[bottom - 6] >= 6:
                return True
    return False
