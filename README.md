# Telegram bot for Miro board

Extremely simply golang telegram bot which notifies about any changes on a board. It checks the content of the board and notices when some difference occurs.

## Install locally for testing

1. Get source code

    ```(bash)
    git clone https://github.com/konstgav/miro-board-telegram-bot/
    cd miro-board-telegram-bot
    ```

2. Set environmental variables

    ```(bash)
    cp .env.example .env
    ```

3. Run with docker

    ```(bash)
    docker build -t miro-telegram-bot .
    docker run -p 7000:7000 miro-telegram-bot
    ```

4. Exposes local network service and share local webhook service

    ```(bash)
    ngrok http 7000
    ```

5. Link your telegram bot with local web service

    ```(bash)
    curl --request POST --url https://api.telegram.org/bot-token/setWebhook --header 'content-type: application/json' --data '{"url": "https-ngrok-public-name"}
    ```

6. It should works. Try to send `/help` message to bot.
