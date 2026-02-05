# Cheapskate 🏠

**Finance tracking as it should be. Not as it has been.**

Cheapskate is a frugal, self-hosted finance tracker built for families who want to keep tabs on their spending without handing over their data to third-party aggregators. It's fast, simple, and built with a modern Go stack.

![Dashboard Preview](https://via.placeholder.com/800x400?text=Cheapskate+Dashboard+Preview)

## Features

- **⚡ Blazing Fast**: Server-side rendered HTML with Go & Templ.
- **📱 Responsive**: Mobile-first design using Tailwind CSS.
- **🔋 Batteries Included**: SQLite database included, no complex setup required.
- **🔒 Private**: Your data stays on your machine.
- **✨ Interactive**: Smooth transitions and SPA-feel using HTMX.

## The Stack

We believe in the power of simplicity and the "BORING" stack:

- **Language**: [Go](https://go.dev)
- **Templating**: [Templ](https://templ.guide)
- **Database**: [SQLite](https://sqlite.org) + [SQLC](https://sqlc.dev)
- **Frontend**: [HTMX](https://htmx.org) + [TailwindCSS](https://tailwindcss.com)

## Getting Started

You can spin up your own instance in seconds.

### Prerequisites

- **Go 1.25+**
- **Make**

### Quick Installation

1.  **Clone the repo**
    ```bash
    git clone https://github.com/calexandrepcjr/cheapskate-finance-tracker.git
    cd cheapskate-finance-tracker
    ```

2.  **Install Tooling**
    We use `sqlc` for type-safe SQL and `templ` for HTML generation.
    ```bash
    make tools
    ```

3.  **Run the Server**
    ```bash
    make run
    ```
    Visit `http://localhost:8080`. That's it!

## Contributing

Contributions are what make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

1.  Fork the Project
2.  Create your Feature Branch (`git checkout -b feature/AmazingFeature`)
3.  Commit your Changes (`git commit -m 'Add some AmazingFeature'`)
4.  Push to the Branch (`git push origin feature/AmazingFeature`)
5.  Open a Pull Request

## Development

Want to hack on Cheapskate? We've optimized the developer experience for you.

### Live Reloading

Start the development server with hot-reload enabled (requires `air`, installed via `make tools`):

```bash
make dev
```

The server will automatically rebuild and restart when you change any `.go`, `.templ`, or `.sql` file.

### Project Structure

```
├── client/
│   ├── assets/       # Static assets (images, css)
│   └── templates/    # Templ components (UI)
├── server/
│   ├── db/           # Database schema & generated queries
│   ├── handlers_*.go # HTTP Handlers
│   └── main.go       # Entrypoint
└── Makefile          # Build recipes
```

## License

Cheapskate is open-source software licensed under the [MIT license](LICENSE).
