/*
 * Cocos2d-x game loading screen with animated dots.
 * Creates splash screen with logo image and pulsing dot animation,
 * restores original background on cleanup. Entry point initializes
 * styles, DOM structure, and animation loop.
 */
(function() {
    function createStyle() {
        return ".cocosLoading{position:absolute;top:0;left:0;width:100%;height:100%;background:#252525}" +
            ".cocosLoading .image{display:block;width:100%;height:85%;background:url(./res/icon.png) no-repeat center; max-width:1000px;background-size: 30% auto; margin:0 auto;animation: animate-scale 0.7s, animate-opacity 0.7s, animate-blur 0.7s, animage-glow 1.2s ease-in-out;}" +
            ".cocosLoading ul{height:5%; width:100%;margin-left:50%;padding-inline-start:0px;transform:translateX(-90px);}" +
            ".cocosLoading span{color:#f05a23;text-align:center;font-size:24px;display:block;width:100%;height:10%;background-size: 30% auto;position: absolute}" +
            ".cocosLoading li{list-style:none;float:left;border-radius:24px;width:24px;height:24px;background:#FFF;margin:5px 0 0 10px}" +
            ".cocosLoading li .ball,.cocosLoading li .unball{background-color:#f05a23;background-image:-moz-linear-gradient(90deg,#f05a23 25%,#f0b73c);background-image:-webkit-linear-gradient(90deg,#f05a23 25%,#f0b73c);width:24px;height:24px;border-radius:50px}" +
            ".cocosLoading li .ball{transform:scale(0);-moz-transform:scale(0);-webkit-transform:scale(0);animation:showDot 1s linear forwards;-moz-animation:showDot 1s linear forwards;-webkit-animation:showDot 1s linear forwards}" +
            ".cocosLoading li .unball{transform:scale(1);-moz-transform:scale(1);-webkit-transform:scale(1);animation:hideDot 1s linear forwards;-moz-animation:hideDot 1s linear forwards;-webkit-animation:hideDot 1s linear forwards}" +
            "@keyframes showDot{0%{transform:scale(0,0)}100%{transform:scale(1,1)}}" +
            "@-moz-keyframes showDot{0%{-moz-transform:scale(0,0)}100%{-moz-transform:scale(1,1)}}" +
            "@-webkit-keyframes showDot{0%{-webkit-transform:scale(0,0)}100%{-webkit-transform:scale(1,1)}}" +
            "@keyframes hideDot{0%{transform:scale(1,1)}100%{transform:scale(0,0)}}" +
            "@-moz-keyframes hideDot{0%{-moz-transform:scale(1,1)}100%{-moz-transform:scale(0,0)}}" +
            "@-webkit-keyframes hideDot{0%{-webkit-transform:scale(1,1)}100%{-webkit-transform:scale(0,0)}}" +
            "@keyframes animate-scale{0% {transform:scale(0.2);} 25% {transform:scale(1.2);} 100% {transform:scale(1.0);}}" +
            "@keyframes animate-opacity{0% {opacity: 0.2;} 100% {opacity: 1;}}" +
            "@keyframes animate-blur {0%{-webkit-filter: blur(5px);filter: blur(5px);filter: drop-shadow(16px 16px 20px rgb(247, 148, 36))}100% {-webkit-filter: blur(0px);filter: blur(0px);filter: drop-shadow(0px 0px 0px (247, 148, 36))}}" +
            "@keyframes animate-glow {0% {filter: drop-shadow(16px 16px 20px rgb(247, 148, 36))}100%{filter: drop-shadow(5px 5px 5px (247, 148, 36))}}";
    }

    function createDom(id = "cocosLoading", dotCount = 5) {
        const container = document.createElement("div");
        container.className = "cocosLoading";
        container.id = id;

        const image = document.createElement("div");
        image.className = "image";
        container.appendChild(image);

        const ul = document.createElement("ul");
        const dots = [];

        for (let i = 0; i < dotCount; i++) {
            const li = document.createElement("li");
            dots.push({
                ball: document.createElement("div"),
                halo: null
            });
            li.appendChild(dots[dots.length - 1].ball);
            ul.appendChild(li);
        }

        const span = document.createElement("span");
        span.innerHTML = "LOADING...";

        container.appendChild(ul);
        container.appendChild(span);
        document.body.appendChild(container);

        return dots;
    }

    function startAnimation(dots, onCheckContinue) {
        let index = 0;
        let expanding = true;
        let delay = 300;

        function animation() {
            setTimeout(() => {
                if (onCheckContinue && !onCheckContinue()) {
                    return;
                }

                const dot = dots[index];
                dot.ball.className = expanding ? "ball" : "unball";

                index++;
                if (index >= dots.length) {
                    expanding = !expanding;
                    index = 0;
                    delay = 1000;
                } else {
                    delay = 300;
                }

                animation();
            }, delay);
        }

        animation();
    }

    (function() {
        const originalBackground = document.body.style.background;
        document.body.style.background = "#000";

        const style = document.createElement("style");
        style.type = "text/css";
        style.innerHTML = createStyle();
        document.head.appendChild(style);

        const dots = createDom();
        startAnimation(dots, function() {
            const element = document.getElementById("cocosLoading");
            if (!element) {
                document.body.style.background = originalBackground;
            }
            return !!element;
        });
    })();
})();
