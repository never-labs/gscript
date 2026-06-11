"""Packaging metadata for the provider-free FinRobot translation examples."""

from setuptools import setup


BASE_REQUIREMENTS = [
    "leia>=0.1.0",
]

EXTRAS_REQUIRE = {
    "web": [
        "flask>=3.0",
        "gunicorn>=22.0",
    ],
    "finance": [
        "yfinance>=0.2",
        "pandas>=2.2",
    ],
    "documents": [
        "pypdf>=4.0",
        "beautifulsoup4>=4.12",
    ],
    "charts": [
        "plotly>=5.20",
        "kaleido>=0.2",
    ],
    "cloud": [
        "google-cloud-logging>=3.10",
    ],
}

EXTRAS_REQUIRE["all"] = [
    f"leia-finrobot-translation[{','.join(EXTRAS_REQUIRE.keys())}]",
]


setup(
    name="leia-finrobot-translation",
    version="0.1.0",
    description="Provider-free FinRobot translation examples for the Leia AI dialect",
    python_requires=">=3.10",
    install_requires=BASE_REQUIREMENTS,
    extras_require=EXTRAS_REQUIRE,
    py_modules=["package_deploy_run_web_app"],
    entry_points={
        "console_scripts": [
            "finrobot-translation-web=package_deploy_run_web_app:main",
        ],
    },
)
