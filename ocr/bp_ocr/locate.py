"""Find the LCD rim and order its corners using the position of the start key."""

import cv2
import numpy as np

from .orient import find_buttons


def refine(corners, contour):
    """Fit each straight section of the rounded rim, intersect adjacent lines."""
    points = contour.reshape(-1, 2)
    lines = []
    for p, q in zip(corners, np.roll(corners, -1, axis=0)):
        edge = q - p
        length = np.linalg.norm(edge)
        normal = np.array([-edge[1], edge[0]]) / length
        along = (points - p) @ edge / length**2
        near = (
            (along > 0.13)
            & (along < 0.87)
            & (abs((points - p) @ normal) < 0.055 * length)
        )
        if near.sum() < 8:
            return corners
        vx, vy, x, y = cv2.fitLine(points[near], cv2.DIST_HUBER, 0, 0.01, 0.01).ravel()
        lines.append([-vy, vx, vy * x - vx * y])
    hits = np.cross(np.roll(lines, 1, axis=0), lines)
    if np.any(abs(hits[:, 2]) < 0.2):
        return corners
    return np.float32(hits[:, :2] / hits[:, 2:])


def contours_of_panel(image):
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY)
    filtered = cv2.bilateralFilter(gray, 5, 25, 10)
    edges = cv2.Canny(filtered, 35, 100)
    edges = cv2.morphologyEx(edges, cv2.MORPH_CLOSE, np.ones((3, 3), np.uint8))
    contours, _ = cv2.findContours(edges, cv2.RETR_LIST, cv2.CHAIN_APPROX_NONE)
    return contours


def locate(image):
    keys, frame = find_buttons(image)
    if frame is not None:
        corners = keyed_rim(image, frame)
        if corners is not None:
            return corners
    if not keys:
        return None
    frame_area = image.shape[0] * image.shape[1]
    best_score, best = 0.0, None
    for contour in contours_of_panel(image):
        area = abs(cv2.contourArea(contour))
        if not 0.055 * frame_area < area < 0.75 * frame_area:
            continue
        hull = cv2.convexHull(contour)
        polygon = cv2.approxPolyDP(hull, 0.025 * cv2.arcLength(hull, True), True)
        if len(polygon) == 4:
            quad = polygon.reshape(4, 2).astype(np.float32)
        else:
            quad = cv2.boxPoints(cv2.minAreaRect(hull))
        if area < 0.85 * abs(cv2.contourArea(quad)):
            continue
        if cv2.contourArea(quad, oriented=True) < 0:
            quad = quad[::-1]
        quad = refine(quad, contour)
        for start in range(4):
            corners = np.roll(quad, -start, axis=0)
            top = corners[1] - corners[0]
            down = (corners[2] + corners[3] - corners[0] - corners[1]) / 2
            aspect = np.linalg.norm(top) / np.linalg.norm(down)
            if not 1.4 < aspect < 2.75:
                continue
            basis = np.column_stack((top, down))
            for center, key_area in keys:
                if not 0.015 < key_area / area < 0.16:
                    continue
                x, y = np.linalg.solve(basis, center - corners[0])
                if not 0.2 < x < 0.78 or not -0.65 < y < -0.08:
                    continue
                score = 1 - abs(x - 0.50) - abs(y + 0.32) - 0.4 * abs(aspect - 1.90)
                if score > best_score:
                    best_score, best = score, corners
    return best


def rim_line(gray, horizontal, intercepts, slopes, span, pivot, polarity):
    # Compare both sides of each proposed line. A specular patch or a finger
    # can hide part of the rim, so limit the contribution of any one sample.
    along = np.linspace(*span, 100, dtype=np.float32)
    cross = intercepts[:, None, None] + slopes[None, :, None] * (along - pivot)
    cross = np.rint(cross * 96).astype(int) + (0 if horizontal else 300)
    along = np.rint(along * 96).astype(int) + (300 if horizontal else 0)
    view = gray if horizontal else gray.T
    outside, inside = (
        view[cross - 2 * polarity, along],
        view[cross + 2 * polarity, along],
    )
    valid = (outside >= 0) & (inside >= 0)
    contrasts = np.clip(outside - inside, -25, 70) * valid
    counts = valid.sum(axis=-1)
    scores = contrasts.sum(axis=-1) / np.maximum(counts, 40)
    i, j = np.unravel_index(scores.argmax(), scores.shape)
    k = slopes[j]
    b = intercepts[i] - k * pivot
    return ([-k, 1, -b] if horizontal else [1, -k, -b]), scores[i, j]


def keyed_rim(image, frame):
    center, basis = frame
    # Work in units of start-key width. The projected top corners sit about
    # 1.9 key widths to either side of its centre on this model.
    affine = np.column_stack((basis / 96, center - basis[:, 0] * 300 / 96))
    gray = cv2.cvtColor(image, cv2.COLOR_BGR2GRAY).astype(np.float32)
    patch = cv2.warpAffine(
        gray,
        affine,
        (600, 400),
        flags=cv2.WARP_INVERSE_MAP | cv2.INTER_LINEAR,
        borderValue=-255,
    )

    def line(horizontal, lo, hi, slopes, span, pivot, polarity):
        return rim_line(
            patch, horizontal, np.arange(lo, hi, 0.012), slopes, span, pivot, polarity
        )

    mild = np.arange(-0.19, 0.20, 0.008)
    top, top_strength = line(True, 0.69, 0.98, mild, (-1.67, 1.67), 0, 1)
    bottom, bottom_strength = line(True, 2.52, 3.55, mild, (-1.69, 1.69), 0, -1)
    left, left_strength = line(
        False, -2.01, -1.75, np.arange(-0.42, 0.23, 0.012), (1.05, 2.50), 0.80, 1
    )
    right, right_strength = line(
        False, 1.75, 2.03, np.arange(-0.23, 0.42, 0.012), (1.05, 2.50), 0.80, -1
    )
    if (
        min(top_strength, bottom_strength) < 11
        or max(left_strength, right_strength) < 14
    ):
        return None
    # If one edge is outside the photo, extrapolate from the visible rim and
    # the fixed top width. Perspective makes the bottom a little wider.
    if left_strength < 12:
        slope = -right[1] - 0.22
        left = [1, -slope, 1.92 + 0.80 * slope]
    if right_strength < 12:
        slope = -left[1] + 0.22
        right = [1, -slope, -1.92 + 0.80 * slope]
    lines = np.array([top, right, bottom, left])
    hits = np.cross(np.roll(lines, 1, axis=0), lines)
    return np.float32(hits[:, :2] / hits[:, 2:] @ basis.T + center)
