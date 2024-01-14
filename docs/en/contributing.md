# Contribution Guidelines

Welcome to the CSGHub Server project! We appreciate your interest in contributing to this open-source project.

CSGHub Server is the open-source backend part of the CSGHub, a large-scale model asset management platform. It provides functionalities for managing large-scale model assets, including models and datasets, through a REST API.

## Contribution Workflow

To contribute to the project, follow the "fork and pull request" workflow outlined below. Please don't push changes directly to this repository unless you are a maintainer.

1. Fork the repository on GitHub (https://github.com/OpenCSGs/csghub-server) to your own account.
2. Clone your forked repository, make modifications or improvements.
3. Create a new branch in your forked repository to hold your changes. It's recommended to create a new branch based on the main branch.
4. Make your modifications and improvements on the new branch.
5. When you are done with your changes, submit a Pull Request (PR) to the main branch of the original repository.
6. The maintainers will review your Pull Request (PR) and provide feedback and engage in discussions.
7. After necessary modifications and discussions, your PR will be merged into the main branch.

Ensure your contributions meet the following requirements:

- Code style should be consistent with the project.
- New features or improvements should have appropriate tests.
- Documentation additions or modifications should be clear and understandable for other developers.

## Reporting Issues and Making Suggestions

If you find issues, improvements, or feature requests, report them on our [Issues page](https://github.com/OpenCSGs/csghub-server/issues). We regularly review and respond to your concerns.

When reporting issues or making suggestions, follow these guidelines:

- Provide detailed information, specifying where the issue occurs, the type of error, and relevant code snippets. Generic descriptions like "something is not working" are not helpful. Provide code snippets and context information to help us reproduce and identify the issue.
- If you need to include large sections of code, logs, or trace information, use the `<details>` and `</details>` tags to wrap them. This allows collapsing content for better readability and tracking. You can refer to [this link](https://developer.mozilla.org/en/docs/Web/HTML/Element/details) for more information on using the `<details>` tag and folding content in HTML.

## Tag Descriptions

We use tags to categorize and classify issues and pull requests. For detailed tag descriptions, please refer to [this page](https://github.com/OpenCSGs/csghub-server/labels).

## Local Development
You can use `docker-compose` to launch the CSGHub Server project. After launching, it will start the CSGHub Server project, which is a backend project providing API support for [CSGHub](https://github.com/OpenCSGs/CSGHub). You can also configure and compile locally based on the [configuration documentation](config.md) and [database migration documentation](migration.md).

Thank you for contributing to the CSGHub Server project! We look forward to your participation and suggestions.
