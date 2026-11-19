"""Estimate the light background behind dark LCD strokes at rectified size."""

import cv2
import numpy as np


def enhance(lcd):
    gray = cv2.cvtColor(lcd, cv2.COLOR_BGR2GRAY)
    # Opening in the inverted image removes strokes brighter than its background.
    kernel = cv2.getStructuringElement(cv2.MORPH_RECT, (39, 39))
    light = 255 - cv2.morphologyEx(255 - gray, cv2.MORPH_OPEN, kernel)
    normalized = cv2.divide(gray, np.maximum(light, 1), scale=255)
    binary = np.uint8(normalized < 241) * 255
    hsv = cv2.cvtColor(lcd, cv2.COLOR_BGR2HSV)
    skin = cv2.inRange(hsv, (0, 48, 45), (22, 255, 255))
    # Both clipped white (glare) and clipped black (outside the photo) lack evidence.
    unknown = (gray >= 250) | (gray <= 6) | (skin != 0)
    return gray, light, normalized, binary, unknown
