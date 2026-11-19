"""The small navy start key sits immediately above the middle of the LCD."""

import cv2
import numpy as np


def upright(image, corners):
    # Corners are already ordered relative to the start key.
    dx, dy = corners[1] - corners[0]
    if abs(dx) > abs(dy) and dx > 0:
        return image, corners
    h, w = image.shape[:2]
    x, y = corners.T
    if abs(dx) > abs(dy):
        rotation, points = cv2.ROTATE_180, (w - 1 - x, h - 1 - y)
    elif dy > 0:
        rotation, points = cv2.ROTATE_90_COUNTERCLOCKWISE, (y, w - 1 - x)
    else:
        rotation, points = cv2.ROTATE_90_CLOCKWISE, (h - 1 - y, x)
    return cv2.rotate(image, rotation), np.column_stack(points).astype(np.float32)


def find_buttons(image):
    """Find candidate start keys and use the red key on the right to resolve orientation."""
    hsv = cv2.cvtColor(image, cv2.COLOR_BGR2HSV)
    red = cv2.inRange(hsv, (0, 65, 45), (12, 255, 255)) | cv2.inRange(
        hsv, (166, 65, 45), (179, 255, 255)
    )
    mask = cv2.inRange(hsv, (95, 54, 24), (138, 255, 180))
    contours, _ = cv2.findContours(mask, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    red_contours, _ = cv2.findContours(red, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
    red_keys = []
    for contour in red_contours:
        area = cv2.contourArea(contour)
        (x, y), (w, h), _ = cv2.minAreaRect(contour)
        if (
            area > 0.0003 * mask.size
            and area > 0.60 * w * h
            and max(w, h) > 1.6 * min(w, h)
        ):
            red_keys.append((np.float32([x, y]), max(w, h)))
    keys = []
    best, frame = 0, None
    for contour in contours:
        rect = cv2.minAreaRect(contour)
        (x, y), (w, h), _ = rect
        length = max(w, h)
        area = cv2.contourArea(contour)
        if not 0.0007 * mask.size < area < 0.06 * mask.size or area < 0.62 * w * h:
            continue
        if min(w, h) <= 0 or length >= 3.2 * min(w, h):
            continue
        center = np.float32([x, y])
        keys.append((center, area))
        if length < 1.25 * min(w, h):
            continue
        box = cv2.boxPoints(rect)
        edges = np.roll(box, -1, axis=0) - box
        axis = edges[np.argmax(np.linalg.norm(edges, axis=1))] / length
        for u in (axis, -axis):
            v = np.float32([-u[1], u[0]])
            basis = np.column_stack((u, v)) * length
            for red_center, red_width in red_keys:
                along, across = (red_center - center) @ basis / length**2
                if (
                    not 0.5 < red_width / length < 1.2
                    or not 1.0 < along < 1.9
                    or abs(across) > 0.38
                ):
                    continue
                score = (
                    1
                    - abs(along - 1.42)
                    - abs(across)
                    - 0.4 * abs(red_width / length - 0.85)
                )
                if score > best:
                    best, frame = score, (center, basis)
    return keys, frame
