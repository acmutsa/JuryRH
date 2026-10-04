import { useEffect } from 'react';
import { useNavigate } from 'react-router-dom';
import Cookies from 'universal-cookie';
import Container from '../../components/Container';
import JuryHeader from '../../components/JuryHeader';
import { postRequest } from '../../api';
import { errorAlert } from '../../util';

const JudgeLogin = () => {
    const navigate = useNavigate();

    useEffect(() => {
        async function checkSession() {
            const cookies = new Cookies();
            if (!cookies.get('token')) return;
            const res = await postRequest<OkResponse>('/judge/auth', 'judge', null);
            if (res.status === 401) {
                cookies.remove('token', { path: '/' });
                return;
            }
            if (res.status !== 200) {
                errorAlert(res);
                return;
            }
            if (res.data?.ok === 1) navigate('/judge', { replace: true });
        }
        checkSession();
    }, [navigate]);

    return (
        <>
            <JuryHeader />
            <Container>
                <h1 className="text-3xl text-center">Join Judging</h1>
                <p className="text-light text-center mx-4 my-8">
                    Scan the judging QR code provided by an organizer, then enter your name to
                    start judging. For track judging, scan that track’s QR code.
                </p>
            </Container>
        </>
    );
};

export default JudgeLogin;
