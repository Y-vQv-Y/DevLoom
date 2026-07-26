import copy
import unittest

from normalize_swagger import encode, normalize


class NormalizeSwaggerTest(unittest.TestCase):
    def fixture(self):
        return {
            "swagger": "2.0",
            "definitions": {
                "consts.UserRole": {
                    "type": "string",
                    "enum": ["individual", "admin"],
                    "x-enum-comments": {
                        "UserRoleAdmin": "administrator",
                        "UserRoleIndividual": "individual user",
                    },
                    "x-enum-varnames": ["UserRoleIndividual", "UserRoleAdmin"],
                },
                "types.ConditionStatus": {
                    "type": "integer",
                    "enum": [0, 1],
                    "x-enum-varnames": ["Unknown", "Ready"],
                },
            },
        }

    def test_adds_deterministic_extensions(self):
        document = normalize(self.fixture())
        role = document["definitions"]["consts.UserRole"]
        self.assertEqual(
            role["x-enum-descriptions"], ["individual user", "administrator"]
        )
        self.assertLess(
            list(role).index("x-enum-descriptions"),
            list(role).index("x-enum-varnames"),
        )

        status = document["definitions"]["types.ConditionStatus"]
        self.assertEqual(status["format"], "int32")
        self.assertLess(list(status).index("format"), list(status).index("enum"))

    def test_is_idempotent_and_matches_go_json_escaping(self):
        document = normalize(self.fixture())
        first = encode(document)
        second = encode(normalize(copy.deepcopy(document)))
        self.assertEqual(first, second)
        self.assertIn("individual user", first)

        document["definitions"]["consts.UserRole"]["description"] = "a < b & c > d"
        self.assertIn("a \\u003c b \\u0026 c \\u003e d", encode(document))


if __name__ == "__main__":
    unittest.main()
