/*
 * functions
 *
 * Initializes a full-screen “cocosLoading” overlay: injects CSS, builds DOM nodes, and animates dot classes.
 * Captures and temporarily overrides `document.body.style.background`, restoring it once the loading element is removed.
 * The animation loops via `setTimeout`, toggling each dot between "ball" and "unball" with a longer pause per cycle.
 */
function functions() {
    const createStyle = () =>
        [
            ".cocosLoading{position:absolute;top:0;left:0;width:100%;height:100%;background:#252525}",
            ".cocosLoading .image{display:block;width:100%;height:85%;background:url(./res/icon.png) no-repeat center; max-width:1000px;background-size: 30% auto; margin:0 auto;animation: animate-scale 0.7s, animate-opacity 0.7s, animate-blur 0.7s, animage-glow 1.2s ease-in-out;}",
            ".cocosLoading ul{height:5%; width:100%;margin-left:50%;padding-inline-start:0px;transform:translateX(-90px);}",
            ".cocosLoading span{color:#f05a23;text-align:center;font-size:24px;display:block;width:100%;height:10%;background-size: 30% auto;position: absolute}",
            ".cocosLoading li{list-style:none;float:left;border-radius:24px;width:24px;height:24px;background:#FFF;margin:5px 0 0 10px}",
            ".cocosLoading li .ball,.cocosLoading li .unball{background-color:#f05a23;background-image:-moz-linear-gradient(90deg,#f05a23 25%,#f0b73c);background-image:-webkit-linear-gradient(90deg,#f05a23 25%,#f0b73c);width:24px;height:24px;border-radius:50px}",
            ".cocosLoading li .ball{transform:scale(0);-moz-transform:scale(0);-webkit-transform:scale(0);animation:showDot 1s linear forwards;-moz-animation:showDot 1s linear forwards;-webkit-animation:showDot 1s linear forwards}",
            ".cocosLoading li .unball{transform:scale(1);-moz-transform:scale(1);-webkit-transform:scale(1);animation:hideDot 1s linear forwards;-moz-animation:hideDot 1s linear forwards;-webkit-animation:hideDot 1s linear forwards}",
            "@keyframes showDot{0%{transform:scale(0,0)}100%{transform:scale(1,1)}}",
            "@-moz-keyframes showDot{0%{-moz-transform:scale(0,0)}100%{-moz-transform:scale(1,1)}}",
            "@-webkit-keyframes showDot{0%{-webkit-transform:scale(0,0)}100%{-webkit-transform:scale(1,1)}}",
            "@keyframes hideDot{0%{transform:scale(1,1)}100%{transform:scale(0,0)}}",
            "@-moz-keyframes hideDot{0%{-moz-transform:scale(1,1)}100%{-moz-transform:scale(0,0)}}",
            "@-webkit-keyframes hideDot{0%{-webkit-transform:scale(1,1)}100%{-webkit-transform:scale(0,0)}}",
            "@keyframes animate-scale{0% {transform:scale(0.2);} 25% {transform:scale(1.2);} 100% {transform:scale(1.0);}}",
            "@keyframes animate-opacity{0% {opacity: 0.2;} 100% {opacity: 1;}}",
            "@keyframes animate-blur {0%{-webkit-filter: blur(5px);filter: blur(5px);filter: drop-shadow(16px 16px 20px rgb(247, 148, 36))}100% {-webkit-filter: blur(0px);filter: blur(0px);filter: drop-shadow(0px 0px 0px (247, 148, 36))}}",
            "@keyframes animate-glow {0% {filter: drop-shadow(16px 16px 20px rgb(247, 148, 36))}100%{filter: drop-shadow(5px 5px 5px (247, 148, 36))}}",
        ].join("");

    const createDom = (id = "cocosLoading", dotCount = 5) => {
        const container = document.createElement("div");
        container.className = "cocosLoading";
        container.id = id;

        const image = document.createElement("div");
        image.className = "image";
        container.appendChild(image);

        const list = document.createElement("ul");
        const dots = [];

        for (let i = 0; i < dotCount; i += 1) {
            const li = document.createElement("li");

            const dot = {
                ball: document.createElement("div"),
                halo: null,
            };
            dots.push(dot);

            li.appendChild(dot.ball);
            list.appendChild(li);
        }

        const label = document.createElement("span");
        label.innerHTML = "LOADING...";

        container.appendChild(list);
        container.appendChild(label);
        document.body.appendChild(container);

        return dots;
    };

    const startAnimation = (dots, shouldContinue) => {
        let index = 0;
        let showBall = true;
        let delayMs = 300;

        const animation = () => {
            setTimeout(() => {
                if (shouldContinue && !shouldContinue()) return;

                const dot = dots[index];
                dot.ball.className = showBall ? "ball" : "unball";

                index += 1;

                if (index >= dots.length) {
                    showBall = !showBall;
                    index = 0;
                    delayMs = 1000;
                } else {
                    delayMs = 300;
                }

                animation();
            }, delayMs);
        };

        animation();
    };

    (() => {
        const previousBackground = document.body.style.background;
        document.body.style.background = "#000";

        const styleEl = document.createElement("style");
        styleEl.type = "text/css";
        styleEl.innerHTML = createStyle();
        document.head.appendChild(styleEl);

        const dots = createDom();

        const shouldContinue = () => {
            const loading = document.getElementById("cocosLoading");
            if (!loading) document.body.style.background = previousBackground;
            return !!loading;
        };

        startAnimation(dots, shouldContinue);
    })();
}
