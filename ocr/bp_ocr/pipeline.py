from pathlib import Path

# Load the thread budget before OpenCV and NumPy initialize.
from .config import MAX_EDGE

# isort: split
import cv2
import numpy as np

from .enhance import enhance
from .layout import FIELDS, high_indicator, read_field
from .locate import locate
from .orient import upright
from .warp import warp_lcd


def empty_result():
    return {
        **dict.fromkeys(FIELDS),
        "high_indicator": None,
        "confidence": dict.fromkeys(FIELDS, 0.0),
    }


def save_image(directory, name, image):
    if not cv2.imwrite(str(directory / f"{name}.png"), image):
        raise OSError(f"无法写入调试图 {name}")


def recognize_lcd(lcd, debug_dir=None):
    gray, light, normalized, binary, unknown = enhance(lcd)
    result = empty_result()
    present = []
    if debug_dir is not None:
        overlay = lcd.copy()
        for name, image in (
            ("lcd", lcd),
            ("illumination", light),
            ("normalized", normalized),
            ("binary", binary),
            ("unknown", unknown.astype(np.uint8) * 255),
        ):
            save_image(debug_dir, name, image)

    for field in FIELDS:
        value, confidence, boxes, has_digit = read_field(
            gray, normalized, unknown, field
        )
        result[field] = value
        result["confidence"][field] = confidence
        present.append(has_digit)
        if debug_dir is not None:
            for x, y, w, h in boxes:
                cv2.rectangle(overlay, (x, y), (x + w, y + h), (0, 128, 255), 2)
    if debug_dir is not None:
        save_image(debug_dir, "digits", overlay)
    if not all(present):
        return empty_result()
    result["high_indicator"] = high_indicator(gray, unknown)
    return validate(result)


def validate(result):
    for name, lo, hi in (
        ("systolic", 0, 299),
        ("diastolic", 0, 299),
        ("pulse", 40, 180),
    ):
        value = result[name]
        if value is not None and not lo <= value <= hi:
            result[name] = None
    sys, dia = result["systolic"], result["diastolic"]
    if sys is not None and dia is not None and sys <= dia:
        result["systolic"] = result["diastolic"] = None
    for name in FIELDS:
        if result[name] is None:
            result["confidence"][name] = 0.0
    return result


def read_photo(path, debug_dir=None):
    image = cv2.imread(str(path), cv2.IMREAD_COLOR)
    if image is None:
        raise ValueError(f"无法读取图片 {path}")
    h, w = image.shape[:2]
    scale = min(1.0, MAX_EDGE / max(h, w))
    if scale < 1:
        size = (max(1, round(w * scale)), max(1, round(h * scale)))
        image = cv2.resize(image, size, interpolation=cv2.INTER_AREA)
    if debug_dir is not None:
        debug_dir = Path(debug_dir)
        debug_dir.mkdir(parents=True, exist_ok=True)
    corners = locate(image)
    if corners is None:
        if debug_dir is not None:
            save_image(debug_dir, "location", image)
        return empty_result()
    image, corners = upright(image, corners)
    if debug_dir is not None:
        save_image(debug_dir, "upright", image)
        overlay = image.copy()
        cv2.polylines(overlay, [np.int32(corners)], True, (0, 128, 255), 3)
        save_image(debug_dir, "location", overlay)
    return recognize_lcd(warp_lcd(image, corners), debug_dir)
