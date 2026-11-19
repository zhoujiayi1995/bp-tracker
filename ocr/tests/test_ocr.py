import json
import tempfile
import time
import unittest
from pathlib import Path

import cv2
import numpy as np

from bp_ocr.pipeline import FIELDS, read_photo, recognize_lcd

ROOT = Path(__file__).resolve().parents[1]
PHOTOS = ROOT.parent / "ocr-test-photos"


def draw_reading(lcd, field, text):
    # Standard seven-bar glyph, independent of the decoder's sample points.
    lit = (
        "abcdef",
        "bc",
        "abdeg",
        "abcdg",
        "bcfg",
        "acdfg",
        "acdefg",
        "abc",
        "abcdefg",
        "abcdfg",
    )
    ends = {
        "a": ((0.16, 0.06), (0.84, 0.06)),
        "b": ((0.89, 0.13), (0.89, 0.43)),
        "c": ((0.89, 0.57), (0.89, 0.87)),
        "d": ((0.16, 0.94), (0.84, 0.94)),
        "e": ((0.11, 0.57), (0.11, 0.87)),
        "f": ((0.11, 0.13), (0.11, 0.43)),
        "g": ((0.16, 0.50), (0.84, 0.50)),
    }
    right, top, w, h, pitch = FIELDS[field]
    for i, digit in enumerate(reversed(text)):
        left = right - w - i * pitch
        for bar in lit[int(digit)]:
            a, b = ends[bar]
            p, q = [(round(left + x * w), round(top + y * h)) for x, y in (a, b)]
            cv2.line(lcd, p, q, (50, 50, 50), round(w * 0.18))


def screen(sys="143", dia="83", pulse="63"):
    lcd = np.full((480, 960, 3), 166, np.uint8)
    for name, value in zip(FIELDS, (sys, dia, pulse)):
        draw_reading(lcd, name, value)
    return lcd


class SyntheticTests(unittest.TestCase):
    def test_all_digits_and_field_widths(self):
        for i in range(10):
            values = [str(200 + i), str(i), str(90 + i)]
            with self.subTest(values=values):
                result = recognize_lcd(screen(*values))
                self.assertEqual([result[k] for k in FIELDS], list(map(int, values)))
                self.assertIs(result["high_indicator"], False)
        result = recognize_lcd(screen("99", "80", "180"))
        self.assertEqual([result[k] for k in FIELDS], [99, 80, 180])

    def test_ambiguous_or_obscured_segment(self):
        for color in ((255, 255, 255), (75, 125, 195)):
            with self.subTest(color=color):
                lcd = screen("188", "83", "63")
                # The middle segment distinguishes 8 from 0.
                cv2.rectangle(lcd, (528, 179), (632, 205), color, -1)
                result = recognize_lcd(lcd)
                self.assertIsNone(result["systolic"])
                self.assertEqual(result["confidence"]["systolic"], 0)
                self.assertEqual([result["diastolic"], result["pulse"]], [83, 63])

    def test_hidden_leading_digit(self):
        lcd = screen()
        cv2.rectangle(lcd, (348, 103), (394, 276), (255, 255, 255), -1)
        self.assertIsNone(recognize_lcd(lcd)["systolic"])

    def test_measurement_in_progress_and_limits(self):
        for values, expected in [
            (("142", "", ""), [None, None, None]),
            (("300", "83", "63"), [None, 83, 63]),
            (("100", "100", "63"), [None, None, 63]),
            (("299", "105", "39"), [299, 105, None]),
            (("299", "105", "181"), [299, 105, None]),
            (("299", "0", "40"), [299, 0, 40]),
        ]:
            with self.subTest(values=values):
                result = recognize_lcd(screen(*values))
                self.assertEqual([result[k] for k in FIELDS], expected)

    def test_indicator(self):
        lcd = screen()
        cv2.rectangle(lcd, (750, 161), (930, 212), (76, 76, 76), -1)
        self.assertIs(recognize_lcd(lcd)["high_indicator"], True)
        lcd[147:226, 740:935] = 255
        self.assertIsNone(recognize_lcd(lcd)["high_indicator"])

    def test_no_lcd(self):
        with tempfile.TemporaryDirectory() as tmp:
            image = Path(tmp) / "blank.png"
            cv2.imwrite(str(image), np.full((300, 400, 3), 255, np.uint8))
            result = read_photo(image)
            self.assertTrue(all(result[k] is None for k in FIELDS))


class PhotoTests(unittest.TestCase):
    def setUp(self):
        if not PHOTOS.is_dir() or not any(PHOTOS.glob("*.jpg")):
            self.skipTest("ocr-test-photos is kept locally and is not in the repository")

    def test_all_photos(self):
        expected = json.loads((ROOT / "testdata" / "expected.json").read_text())
        self.assertEqual(len(expected), 26)
        self.assertEqual(set(expected), {f.name for f in PHOTOS.glob("*.jpg")})
        for filename, values in expected.items():
            with self.subTest(photo=filename):
                start = time.perf_counter()
                result = read_photo(PHOTOS / filename)
                elapsed = time.perf_counter() - start
                print(f"{filename}  {elapsed:.3f} s", flush=True)
                self.assertEqual([result[k] for k in FIELDS], values)
                self.assertIs(result["high_indicator"], True)

    def test_printed_labels_alone_do_not_count_as_digits(self):
        lcd = screen("", "", "63")
        for text, xy in (("mmHg", (188, 192)), ("mmHg", (188, 359))):
            cv2.putText(lcd, text, xy, cv2.FONT_HERSHEY_SIMPLEX, 1, (80, 80, 80), 2)
        result = recognize_lcd(lcd)
        self.assertTrue(all(result[k] is None for k in FIELDS))

    def test_same_display_in_four_orientations(self):
        original = cv2.imread(str(next(PHOTOS.glob("*_221_59.jpg"))))
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "unrelated-name.png"
            for rotation in (
                None,
                cv2.ROTATE_90_CLOCKWISE,
                cv2.ROTATE_180,
                cv2.ROTATE_90_COUNTERCLOCKWISE,
            ):
                with self.subTest(rotation=rotation):
                    cv2.imwrite(
                        str(path),
                        original
                        if rotation is None
                        else cv2.rotate(original, rotation),
                    )
                    result = read_photo(path)
                    self.assertEqual([result[k] for k in FIELDS], [143, 83, 63])


if __name__ == "__main__":
    unittest.main()
